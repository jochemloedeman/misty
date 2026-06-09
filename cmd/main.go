package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jochemloedeman/misty/api"
	"github.com/jochemloedeman/misty/auth"
	"github.com/jochemloedeman/misty/clock"
	"github.com/jochemloedeman/misty/db"
	"github.com/jochemloedeman/misty/db/sqlc"
	"github.com/jochemloedeman/misty/monitor"
	"github.com/jochemloedeman/misty/notification"
	"github.com/jochemloedeman/misty/notification/apple"
	"github.com/jochemloedeman/misty/user"
	"github.com/jochemloedeman/misty/weather"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/token"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/sync/errgroup"
)

const (
	shutdownTimeout = 10 * time.Second
	maxBodySize     = 4 << 10
)

func requestFilter(r *http.Request) bool {
	if slices.Contains([]string{"/health", "/metrics"}, r.URL.Path) {
		return false
	}
	return true
}

func runServer(
	ctx context.Context,
	routes *api.API,
	verifier api.TokenVerifier,
	port string,
) error {
	mux := http.NewServeMux()

	requireUser := api.RequireUser(verifier)
	protected := func(h http.HandlerFunc) http.HandlerFunc {
		return requireUser(h).ServeHTTP
	}

	// protected routes
	mux.HandleFunc("GET /monitors", protected(routes.ListMonitors))
	mux.HandleFunc("GET /monitors/{id}", protected(routes.GetMonitor))
	mux.HandleFunc(
		"GET /monitors/{id}/forecasts",
		protected(routes.ListForecasts),
	)
	mux.HandleFunc("POST /monitors", protected(routes.CreateMonitor))
	mux.HandleFunc(
		"POST /monitors/{id}/deactivate",
		protected(routes.SetMonitorStatus(false)),
	)
	mux.HandleFunc(
		"POST /monitors/{id}/activate",
		protected(routes.SetMonitorStatus(true)),
	)
	mux.HandleFunc("DELETE /monitors/{id}", protected(routes.DeleteMonitor))
	mux.HandleFunc("PUT /device", protected(routes.UpdatePushToken))

	// bare route
	mux.HandleFunc("POST /register", routes.Register)
	mux.HandleFunc("POST /token/refresh", routes.TokenRefresh)
	mux.HandleFunc("GET /health", routes.HealthCheck)

	instrumented := otelhttp.NewHandler(
		mux,
		"server",
		otelhttp.WithFilter(requestFilter),
	)
	srv := &http.Server{
		Addr: ":" + port,
		Handler: api.Compose(
			api.RequestLogger,
			api.MaxBodySize(maxBodySize),
		)(instrumented),
	}

	go func() {
		<-ctx.Done()
		slog.InfoContext(ctx, "http server shutting down")

		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			shutdownTimeout,
		)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	slog.InfoContext(ctx, "http server listening", "addr", srv.Addr)

	err := srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func runCycle(
	ctx context.Context,
	refresher *monitor.Refresher,
	notifier *notification.Notifier,
	interval time.Duration,
	horizon monitor.ForecastHorizon,
	clock clock.RealClock,
) error {
	ticker := clock.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := refresher.RefreshAll(ctx, horizon); err != nil {
				slog.ErrorContext(ctx, "refresh error", "error", err)
			}
		case m := <-refresher.RefreshC():
			if err := refresher.RefreshOne(ctx, m, horizon); err != nil {
				slog.ErrorContext(
					ctx,
					"immediate refresh failed",
					"monitor_id",
					m.ID,
					"error",
					err,
				)
			}
		case <-ctx.Done():
			return nil
		}

		if err := notifier.Notify(ctx); err != nil {
			slog.ErrorContext(ctx, "notification error", "error", err)
		}
	}
}

func checkHealth(port string) {
	resp, err := http.Get("http://localhost:" + port + "/health")
	if err != nil {
		os.Exit(1)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		os.Exit(1)
	}
}

func run() (err error) {
	cfg, err := loadConfig()
	if err != nil {
		slog.ErrorContext(
			context.Background(),
			"invalid configuration",
			"error",
			err,
		)
		os.Exit(1)
	}

	slog.SetDefault(slog.New(fanout{
		otelslog.NewHandler("misty"),
		slog.NewTextHandler(
			os.Stderr,
			&slog.HandlerOptions{Level: cfg.LogLevel},
		),
	}))

	slog.InfoContext(context.Background(), "starting misty",
		"port", cfg.Port,
		"log_level", cfg.LogLevel,
		"reconcile_interval", cfg.ReconcileInterval,
		"forecast_horizon", cfg.ForecastHorizon,
	)

	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	otelShutdown, err := setupOTelSDK(ctx)
	if err != nil {
		return fmt.Errorf("failed to setup open telemtry: %w", err)
	}
	defer func() {
		err = errors.Join(err, otelShutdown(ctx))
	}()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	err = db.Migrate(ctx, pool)
	if err != nil {
		return fmt.Errorf("database migration failed: %w", err)
	}

	slog.InfoContext(ctx, "database connected")

	clk := clock.NewRealClock()

	queries := sqlc.New(pool)
	userStore := user.NewStore(queries)
	monitorStore := monitor.NewMonitorStore(queries)

	refresher := monitor.NewRefresher(
		weather.NewForecaster(&http.Client{}),
		monitorStore,
		monitor.NewRunAtomically(pool),
		clk,
	)

	keyRing, err := auth.NewKeyRing(cfg.SigningSecrets, clk.Now)
	if err != nil {
		slog.ErrorContext(ctx, "invalid key ring configuration", "error", err)
		os.Exit(1)
	}
	forecastStore := monitor.NewForecastStore(queries)
	routes := api.New(
		userStore,
		monitorStore,
		forecastStore,
		keyRing,
		refresher.RequestRefresh,
		clk.Now,
		cfg.MonitorLimit,
	)

	var deliverFn func(context.Context, notification.Fog) error
	if cfg.APNS != nil {
		authKey, err := token.AuthKeyFromFile(cfg.APNS.KeyPath)
		if err != nil {
			slog.ErrorContext(ctx, "invalid APNs auth key", "error", err)
			os.Exit(1)
		}
		tok := &token.Token{
			AuthKey: authKey,
			KeyID:   cfg.APNS.KeyID,
			TeamID:  cfg.APNS.TeamID,
		}
		apnsClient := apns2.NewTokenClient(tok)
		if cfg.APNS.Development {
			apnsClient = apnsClient.Development()
		} else {
			apnsClient = apnsClient.Production()
		}
		deliverFn = apple.NewDeliverer(
			apnsClient,
			apple.NewPGTokenResolver(queries),
			cfg.APNS.Topic,
		)
		slog.InfoContext(
			ctx,
			"APNs delivery enabled",
			"topic",
			cfg.APNS.Topic,
			"environment",
			map[bool]string{true: "development", false: "production"}[cfg.APNS.Development],
		)
	} else {
		slog.WarnContext(
			ctx,
			"APNs not configured — notifications will be logged only",
		)
		deliverFn = func(ctx context.Context, notif notification.Fog) error {
			slog.InfoContext(ctx, "notification delivered (no-op)",
				"notification_id", notif.ID,
				"recipient_id", notif.RecipientID,
			)
			return nil
		}
	}

	notifier := notification.NewNotifier(
		notification.NewOutbox(queries),
		deliverFn,
		clk.Now,
	)

	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return runServer(ctx, routes, keyRing, cfg.Port)
	})
	group.Go(func() error {
		return runCycle(
			ctx,
			refresher,
			notifier,
			cfg.ReconcileInterval,
			cfg.ForecastHorizon,
			clk,
		)
	})

	if err := group.Wait(); err != nil {
		slog.ErrorContext(ctx, "application error", "error", err)
		return err
	}
	return nil
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "health" {
		checkHealth("8080")
		return
	}
	if err := run(); err != nil {
		slog.ErrorContext(
			context.Background(),
			"application error",
			"error",
			err,
		)
		os.Exit(1)
	}
}
