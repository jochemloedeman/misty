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
	"github.com/jochemloedeman/misty/clock"
	"github.com/jochemloedeman/misty/db/sqlc"
	"github.com/jochemloedeman/misty/monitor"
	"github.com/jochemloedeman/misty/notifications"
	"github.com/jochemloedeman/misty/users"
	"github.com/jochemloedeman/misty/weather"
	"golang.org/x/sync/errgroup"
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

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: api.RequestLogger(mux),
	}

	go func() {
		<-ctx.Done()
		slog.Info("http server shutting down")

		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
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

func runReconciliation(
	ctx context.Context,
	refresher *monitor.Refresher,
	notifier *notifications.Notifier,
	interval time.Duration,
	horizon monitor.ForecastHorizon,
	clock clock.RealClock,
) error {
	ticker := clock.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			slog.Debug("reconciliation tick")

			err := refresher.RefreshAll(ctx, horizon)
			if err != nil {
				return err
			}

			err = notifier.Notify(ctx)
			if err != nil {
				return err
			}

			slog.Debug("reconciliation complete")
		case <-ctx.Done():
			return nil
		}
	}
}

func main() {
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
	slog.Info("database connected")

	clk := clock.NewRealClock()

	queries := sqlc.New(pool)
	userStore := users.NewUserStore(queries)
	refresher := monitor.NewRefresher(
		weather.NewFakeForecaster(clk, 0.9, 1.),
		monitor.NewMonitorStore(queries),
		monitor.NewRunAtomically(pool),
		clk,
	)

	keyRing, err := users.NewKeyRing(cfg.SigningSecrets, clk.Now)
	if err != nil {
		slog.Error("invalid key ring configuration", "error", err)
		os.Exit(1)
	}
	routes := api.New(userStore, func(uid uuid.UUID) api.MonitorStore {
		return monitor.NewScopedMonitorStore(uid, queries)
	}, keyRing)

	notifier := notifications.NewNotifier(
		notifications.NewOutbox(queries),
		func(ctx context.Context, notif notifications.Notification) error {
			slog.Info(
				"delivering notification",
				"notification_id",
				notif.ID,
				"recipient_id",
				notif.RecipientID,
			)

			return nil
		},
	)

	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return runServer(ctx, routes, keyRing, cfg.Port)
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
