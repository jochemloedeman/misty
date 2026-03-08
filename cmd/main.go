package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jochemloedeman/misty/api"
	"github.com/jochemloedeman/misty/db/sqlc"
	"github.com/jochemloedeman/misty/monitor"
	"github.com/jochemloedeman/misty/monitor/postgres"
	"github.com/jochemloedeman/misty/users"
	userdb "github.com/jochemloedeman/misty/users/postgres"
	"github.com/jochemloedeman/misty/weather"
	"golang.org/x/sync/errgroup"
)

func runServer(ctx context.Context, store api.MonitorStore) error {
	api := api.New(store)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /monitors", api.ListMonitors)
	mux.HandleFunc("GET /monitors/{id}", api.GetMonitor)
	mux.HandleFunc("POST /monitors", api.CreateMonitor)
	mux.HandleFunc("POST /monitors/{id}/deactivate", api.SetMonitorStatus(false))
	mux.HandleFunc("POST /monitors/{id}/activate", api.SetMonitorStatus(true))
	mux.HandleFunc("DELETE /monitors/{id}", api.DeleteMonitor)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: api.RequireUser(mux),
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

func runReconciliation(ctx context.Context, refresher *monitor.Refresher) error {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := refresher.RefreshAll(
				ctx,
				monitor.ForecastHorizon{
					Granularity: time.Hour,
					Steps:       15,
				},
			)
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func main() {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(fmt.Errorf("failed to connect to database: %w", err))
	}
	defer pool.Close()

	queries := sqlc.New(pool)
	monitorStore := postgres.NewMonitorStore(queries)
	refresher := monitor.NewRefresher(
		weather.NewFakeForecaster(1, time.Hour, 0.9, 0.9),
		monitorStore,
		postgres.NewRunAtomically(pool),
	)

	if err := seedDevUser(ctx, queries); err != nil {
		log.Fatal(fmt.Errorf("failed to seed dev user: %w", err))
	}

	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return runServer(ctx, monitorStore)
	})
	group.Go(func() error {
		return runReconciliation(ctx, refresher)
	})
	if err := group.Wait(); err != nil {
		log.Fatal(err)
	}

}
