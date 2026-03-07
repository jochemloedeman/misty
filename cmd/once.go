package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jochemloedeman/misty/db/sqlc"
	"github.com/jochemloedeman/misty/monitor"
	mp "github.com/jochemloedeman/misty/monitor/postgres"
	"github.com/jochemloedeman/misty/users"
	up "github.com/jochemloedeman/misty/users/postgres"
	"github.com/jochemloedeman/misty/weather"
)

func once() {
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create connection pool: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	queries := sqlc.New(pool)
	userStore := up.NewUserStore(queries)
	u, err := userStore.Create(context.Background(), users.User{ID: uuid.New()})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create user: %v\n", err)
		os.Exit(1)
	}

	monitorStore := mp.NewMonitorStore(queries)
	m := monitor.NewMonitor(u.ID, monitor.Location{})
	if _, err = monitorStore.Create(context.Background(), m); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create monitor: %v\n", err)
		os.Exit(1)
	}

	refresher := monitor.NewRefresher(
		weather.NewFakeForecaster(1, time.Hour, 0.9, 0.9),
		monitorStore,
		mp.NewRunAtomically(pool),
	)
	if err := refresher.RefreshAll(context.Background(), monitor.ForecastHorizon{
		Granularity: time.Hour,
		Steps:       15,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to refresh monitors: %v\n", err)
		os.Exit(1)
	}

}
