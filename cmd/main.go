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

	"github.com/exaring/otelpgx"
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
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sideshow/apns2"
	"github.com/sideshow/apns2/token"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/sync/errgroup"
)

const (
	shutdownTimeout = 5 * time.Second
	maxBodySize     = 4 << 10
	bufferSize      = 8
)

var skipPaths = []string{"/health", "/metrics"}

func requestFilter(r *http.Request) bool {
	if slices.Contains(skipPaths, r.URL.Path) {
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
	mux.HandleFunc("GET /monitors/{id}/forecasts", protected(routes.ListForecasts))
	mux.HandleFunc("POST /monitors", protected(routes.CreateMonitor))
	mux.HandleFunc("POST /monitors/{id}/deactivate", protected(routes.SetMonitorStatus(false)))
	mux.HandleFunc("POST /monitors/{id}/activate", protected(routes.SetMonitorStatus(true)))
	mux.HandleFunc("DELETE /monitors/{id}", protected(routes.DeleteMonitor))
	mux.HandleFunc("PUT /device", protected(routes.UpdatePushToken))

	// bare route
	mux.HandleFunc("POST /register", routes.Register)
	mux.HandleFunc("POST /token/refresh", routes.TokenRefresh)
	mux.HandleFunc("GET /health", routes.HealthCheck)
	mux.Handle("GET /metrics", promhttp.Handler())

	instrumented := otelhttp.NewHandler(mux, "server", otelhttp.WithFilter(requestFilter))
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: api.Compose(api.RequestLogger, api.MaxBodySize(maxBodySize))(instrumented),
	}

	go func() {
		<-ctx.Done()
		slog.InfoContext(ctx, "http server shutting down")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
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

func runRefreshLoop(
	ctx context.Context,
	refresher *monitor.Refresher,
	refreshQueue *Queue[monitor.Monitor],
	notifyQueue *Queue[notification.Queued],
	interval time.Duration,
	clock clock.RealClock,
) error {
	ticker := clock.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := refresher.RefreshAll(ctx); err != nil {
				slog.ErrorContext(ctx, "refresh error", "error", err)
			}
		case envelope := <-refreshQueue.C():
			ctx := envelope.Context(ctx)
			queued, err := refresher.Refresh(ctx, envelope.payload)
			if err != nil {
				slog.ErrorContext(ctx, "immediate refresh failed", "monitor_id", envelope.payload.ID, "error", err)
			} else if queued != nil {
				notifyQueue.Enqueue(ctx, *queued)
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func runNotifyLoop(
	ctx context.Context,
	notifier *notification.Notifier,
	notifyQueue *Queue[notification.Queued],
	interval time.Duration,
	clock clock.RealClock,
) error {
	ticker := clock.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := notifier.Notify(ctx); err != nil {
				slog.ErrorContext(ctx, "notification error", "error", err)
			}
		case envelope := <-notifyQueue.C():
			ctx := envelope.Context(ctx)
			if err := notifier.NotifyOne(ctx, envelope.payload.ID); err != nil {
				slog.ErrorContext(
					ctx,
					"immediate notification failed",
					"notification_id",
					envelope.payload.ID,
					"error",
					err,
				)
			}
		case <-ctx.Done():
			return nil
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
		return fmt.Errorf("invalid configuration: %w", err)
	}

	slog.SetDefault(slog.New(fanout{
		otelslog.NewHandler("github.com/jochemloedeman/misty"),
		slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}),
	}))

	slog.InfoContext(context.Background(), "starting misty",
		"port", cfg.Port,
		"log_level", cfg.LogLevel,
		"refresh_interval", cfg.RefreshInterval,
		"notify_interval", cfg.NotifyInterval,
		"forecast_horizon", cfg.ForecastHorizon,
	)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	otelShutdown, err := setupOTelSDK(ctx)
	if err != nil {
		return fmt.Errorf("failed to setup open telemetry: %w", err)
	}
	defer func() {
		sctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		err = errors.Join(err, otelShutdown(sctx))
	}()

	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("parse database config: %w", err)
	}
	poolCfg.ConnConfig.Tracer = otelpgx.NewTracer()

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return fmt.Errorf("creating database pool: %w", err)
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

	refreshQueue := NewQueue[monitor.Monitor]("refresh", bufferSize)
	notifyQueue := NewQueue[notification.Queued]("notify", bufferSize)
	refresher, err := monitor.NewRefresher(
		weather.NewForecaster(&http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}),
		monitor.NewRunAtomically(pool),
		clk,
		monitorStore,
		cfg.ForecastHorizon,
	)
	if err != nil {
		return fmt.Errorf("creating refresher: %w", err)
	}

	keyRing, err := auth.NewKeyRing(cfg.SigningSecrets, clk.Now)
	if err != nil {
		return fmt.Errorf("creating key ring: %w", err)
	}
	forecastStore := monitor.NewForecastStore(queries)
	monitorService := monitor.NewService(monitorStore, refreshQueue.Enqueue, cfg.MonitorLimit)
	routes := api.New(
		userStore,
		monitorService,
		forecastStore,
		keyRing,
		clk.Now,
	)

	var deliverFn func(context.Context, notification.Fog) error
	if cfg.APNS != nil {
		authKey, err := token.AuthKeyFromFile(cfg.APNS.KeyPath)
		if err != nil {
			return fmt.Errorf("invalid auth key: %w", err)
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
		apnsClient.HTTPClient.Transport = otelhttp.NewTransport(apnsClient.HTTPClient.Transport)
		deliverFn = apple.NewDeliverer(apnsClient, apple.NewPGTokenResolver(queries), cfg.APNS.Topic)
		env := "production"
		if cfg.APNS.Development {
			env = "development"
		}
		slog.InfoContext(ctx, "APNs delivery enabled", "topic", cfg.APNS.Topic, "environment", env)
	} else {
		slog.WarnContext(ctx, "APNs not configured — notifications will be logged only")
		deliverFn = func(ctx context.Context, notif notification.Fog) error {
			slog.InfoContext(ctx, "notification delivered (no-op)",
				"notification_id", notif.ID,
				"recipient_id", notif.RecipientID,
			)
			return nil
		}
	}

	notifier, err := notification.NewNotifier(notification.NewOutbox(queries), deliverFn, clk.Now)
	if err != nil {
		return fmt.Errorf("failed to create notifier: %w", err)
	}

	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return runServer(ctx, routes, keyRing, cfg.Port)
	})
	group.Go(func() error {
		return runRefreshLoop(
			ctx,
			refresher,
			refreshQueue,
			notifyQueue,
			cfg.RefreshInterval,
			clk,
		)
	})
	group.Go(func() error {
		return runNotifyLoop(
			ctx,
			notifier,
			notifyQueue,
			cfg.NotifyInterval,
			clk,
		)
	})

	if err := group.Wait(); err != nil {
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
		slog.ErrorContext(context.Background(), "application error", "error", err)
		os.Exit(1)
	}
}
