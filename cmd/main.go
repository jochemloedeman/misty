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
	"github.com/jochemloedeman/misty/monitor/postgres"
	"github.com/jochemloedeman/misty/users"
	userdb "github.com/jochemloedeman/misty/users/postgres"
	"github.com/jochemloedeman/misty/weather"
	"golang.org/x/sync/errgroup"
)

type Clock interface {
	Now() time.Time
	NewTicker(d time.Duration) *time.Ticker
}

func runServer(ctx context.Context, store api.MonitorStore, port string) error {
	routes := api.New(store)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /monitors", routes.ListMonitors)
	mux.HandleFunc("GET /monitors/{id}", routes.GetMonitor)
	mux.HandleFunc("POST /monitors", routes.CreateMonitor)
	mux.HandleFunc("POST /monitors/{id}/deactivate", routes.SetMonitorStatus(false))
	mux.HandleFunc("POST /monitors/{id}/activate", routes.SetMonitorStatus(true))
	mux.HandleFunc("DELETE /monitors/{id}", routes.DeleteMonitor)

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: api.RequestLogger(api.RequireUser(mux)),
	}

	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()

	err := srv.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func seedDevUser(ctx context.Context, q *sqlc.Queries) error {
	devUser := users.User{ID: uuid.MustParse("00000000-0000-0000-0000-000000000001")}
	userStore := userdb.NewUserStore(q)
	return userStore.Ensure(ctx, devUser)
}

func runReconciliation(ctx context.Context, refresher *monitor.Refresher, interval time.Duration, horizon monitor.TimeHorizon, clock Clock) error {
	ticker := clock.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := refresher.RefreshAll(ctx, horizon)
			if err != nil {
				return err
			}
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

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	clock := clock.NewFastClock(60)

	queries := sqlc.New(pool)
	monitorStore := postgres.NewMonitorStore(queries)
	refresher := monitor.NewRefresher(
		weather.NewFakeForecaster(clock, 0.9, 0.9),
		monitorStore,
		postgres.NewRunAtomically(pool),
		clock,
	)

	if err := seedDevUser(ctx, queries); err != nil {
		slog.Error("failed to seed dev user", "error", err)
		os.Exit(1)
	}

	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return runServer(ctx, monitorStore, cfg.Port)
	})
	group.Go(func() error {
		return runReconciliation(
			ctx,
			refresher,
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
