package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
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
	"golang.org/x/sync/errgroup"
)

const (
	shutdownTimeout = 10 * time.Second
	maxBodySize     = 4 << 10
)

func runServer(
	ctx context.Context,
	routes *api.API,
	verifier api.TokenVerifier,
	port string,
) error {
	authenticated := http.NewServeMux()
	authenticated.HandleFunc("GET /monitors", routes.ListMonitors)
	authenticated.HandleFunc("GET /monitors/{id}", routes.GetMonitor)
	authenticated.HandleFunc("POST /monitors", routes.CreateMonitor)
	authenticated.HandleFunc(
		"POST /monitors/{id}/deactivate",
		routes.SetMonitorStatus(false),
	)
	authenticated.HandleFunc(
		"POST /monitors/{id}/activate",
		routes.SetMonitorStatus(true),
	)
	authenticated.HandleFunc("DELETE /monitors/{id}", routes.DeleteMonitor)

	mux := http.NewServeMux()
	mux.Handle("/", api.RequireUser(verifier)(authenticated))
	mux.HandleFunc("POST /register", routes.Register)
	mux.HandleFunc("POST /token/refresh", routes.TokenRefresh)
	mux.HandleFunc("GET /health", routes.HealthCheck)

	srv := &http.Server{
		Addr: ":" + port,
		Handler: api.Compose(
			api.RequestLogger,
			api.MaxBodySize(maxBodySize),
		)(mux),
	}

	go func() {
		<-ctx.Done()
		slog.Info("http server shutting down")

		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			shutdownTimeout,
		)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	slog.Info("http server listening", "addr", srv.Addr)

	err := srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func doReconciliation(
	ctx context.Context,
	refresher *monitor.Refresher,
	notifier *notification.Notifier,
	horizon monitor.ForecastHorizon,
) error {
	slog.Debug("starting reconciliation")

	err := refresher.RefreshAll(ctx, horizon)
	if err != nil {
		return err
	}

	err = notifier.Notify(ctx)
	if err != nil {
		return err
	}

	slog.Debug("reconciliation complete")
	return nil
}

func runReconciliation(
	ctx context.Context,
	refresher *monitor.Refresher,
	notifier *notification.Notifier,
	interval time.Duration,
	horizon monitor.ForecastHorizon,
	clock clock.RealClock,
) error {
	ticker := clock.NewTicker(interval)
	defer ticker.Stop()

	if err := doReconciliation(ctx, refresher, notifier, horizon); err != nil {
		slog.Error("reconciliation error", "error", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := doReconciliation(ctx, refresher, notifier, horizon); err != nil {
				slog.Error("reconciliation error", "error", err)
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

func main() {
	if len(os.Args) > 1 && os.Args[1] == "health" {
		checkHealth("8080")
		return
	}

	cfg, err := loadConfig()
	if err != nil {
		slog.Error("invalid configuration", "error", err)
		os.Exit(1)
	}

	slog.SetDefault(
		slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: cfg.LogLevel,
		})),
	)
	slog.Info("starting misty",
		"port", cfg.Port,
		"log_level", cfg.LogLevel,
		"reconcile_interval", cfg.ReconcileInterval,
		"forecast_horizon", cfg.ForecastHorizon,
	)

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	err = db.Migrate(ctx, pool)
	if err != nil {
		slog.Error("database migration failed", "error", err)
		os.Exit(1)
	}

	slog.Info("database connected")

	clk := clock.NewRealClock()

	queries := sqlc.New(pool)
	userStore := user.NewStore(queries)

	refresher := monitor.NewRefresher(
		weather.NewForecaster(&http.Client{}),
		monitor.NewMonitorStore(queries),
		monitor.NewRunAtomically(pool),
		clk,
	)

	keyRing, err := auth.NewKeyRing(cfg.SigningSecrets, clk.Now)
	if err != nil {
		slog.Error("invalid key ring configuration", "error", err)
		os.Exit(1)
	}
	routes := api.New(userStore, func(uid uuid.UUID) api.MonitorStore {
		return monitor.NewScopedMonitorStore(uid, queries)
	}, keyRing)

	var deliverFn func(context.Context, notification.Notification) error
	if cfg.APNS != nil {
		authKey, err := token.AuthKeyFromFile(cfg.APNS.KeyPath)
		if err != nil {
			slog.Error("invalid APNs auth key", "error", err)
			os.Exit(1)
		}
		tok := &token.Token{
			AuthKey: authKey,
			KeyID:   cfg.APNS.KeyID,
			TeamID:  cfg.APNS.TeamID,
		}
		apnsClient := apns2.NewTokenClient(tok).Production()
		deliverFn = apple.NewDeliverer(
			apnsClient,
			apple.NewPGTokenResolver(queries),
			cfg.APNS.Topic,
		)
		slog.Info("APNs delivery enabled", "topic", cfg.APNS.Topic)
	} else {
		slog.Warn("APNs not configured — notifications will be logged only")
		deliverFn = func(_ context.Context, notif notification.Notification) error {
			slog.Info("notification delivered (no-op)",
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

	var verifier api.TokenVerifier = keyRing
	if cfg.DevUserID != nil {
		slog.Warn("authentication disabled — using fixed dev user", "user_id", cfg.DevUserID)
		verifier = api.NewDevVerifier(*cfg.DevUserID)

		devUser := user.User{ID: *cfg.DevUserID}
		err := userStore.Ensure(ctx, devUser)
		if err != nil {
			slog.Error("create dev user", "error", err)
			os.Exit(1)
		}
		userStore.Ensure(ctx, devUser)
	}

	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return runServer(ctx, routes, verifier, cfg.Port)
	})
	group.Go(func() error {
		return runReconciliation(
			ctx,
			refresher,
			notifier,
			cfg.ReconcileInterval,
			cfg.ForecastHorizon,
			clk,
		)
	})

	if err := group.Wait(); err != nil {
		slog.Error("application error", "error", err)
		os.Exit(1)
	}
}
