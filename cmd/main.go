package main

import (
	"context"
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

func runServer(ctx context.Context, routes *api.API, port string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /monitors", routes.ListMonitors)
	mux.HandleFunc("GET /monitors/{id}", routes.GetMonitor)

	mux.HandleFunc("POST /monitors", routes.CreateMonitor)
	mux.HandleFunc("POST /monitors/{id}/deactivate", routes.SetMonitorStatus(false))
	mux.HandleFunc("POST /monitors/{id}/activate", routes.SetMonitorStatus(true))

	mux.HandleFunc("DELETE /monitors/{id}", routes.DeleteMonitor)

	mux.HandleFunc("POST /register", routes.Register)
	mux.HandleFunc("POST /token/refresh", routes.TokenRefresh)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: api.RequestLogger(api.RequireUser(mux)),
	}

	go func() {
		<-ctx.Done()
		slog.Info("http server shutting down")
		_ = srv.Shutdown(context.Background())
	}()

	slog.Info("http server listening", "addr", srv.Addr)
	err := srv.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func seedDevUser(ctx context.Context, q *sqlc.Queries) error {
	devUser := users.User{ID: uuid.MustParse("00000000-0000-0000-0000-000000000001")}
	userStore := users.NewUserStore(q)
	return userStore.Ensure(ctx, devUser)
}

func runReconciliation(
	ctx context.Context,
	refresher *monitor.Refresher,
	notifier *notifications.Notifier,
	interval time.Duration,
	horizon monitor.TimeHorizon,
	clock clock.FastClock,
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

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: cfg.LogLevel,
	})))
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

	clock := clock.NewFastClock(1. / 360)

	queries := sqlc.New(pool)
	userStore := users.NewUserStore(queries)
	refresher := monitor.NewRefresher(
		weather.NewFakeForecaster(clock, 0.9, 1.),
		monitor.NewMonitorStore(queries),
		monitor.NewRunAtomically(pool),
		clock,
	)

	if err := seedDevUser(ctx, queries); err != nil {
		slog.Error("failed to seed dev user", "error", err)
		os.Exit(1)
	}

	routes := api.New(userStore, func(uid uuid.UUID) api.MonitorStore {
		return monitor.NewScopedMonitorStore(uid, queries)
	})

	notifier := notifications.NewNotifier(
		notifications.NewOutbox(queries),
		func(ctx context.Context, notif notifications.Notification) error {
			slog.Info("delivering notification", "notification_id", notif.ID, "recipient_id", notif.RecipientID)
			return nil
		},
	)

	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return runServer(ctx, routes, cfg.Port)
	})
	group.Go(func() error {
		return runReconciliation(
			ctx,
			refresher,
			notifier,
			cfg.ReconcileInterval,
			cfg.ForecastHorizon,
			clock,
		)
	})
	if err := group.Wait(); err != nil {
		slog.Error("application error", "error", err)
		os.Exit(1)
	}

}
