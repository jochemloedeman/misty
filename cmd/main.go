package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jochemloedeman/misty/db/sqlc"
	"github.com/jochemloedeman/misty/monitor/postgres"
	"github.com/jochemloedeman/misty/server"
)

func runServer() error {
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		return fmt.Errorf("unable to create connection pool: %w", err)
	}
	defer pool.Close()

	queries := sqlc.New(pool)

	srv := server.New(postgres.NewMonitorStore(queries))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /monitors", srv.ListMonitors)
	mux.HandleFunc("GET /monitors/{id}", srv.GetMonitor)
	mux.HandleFunc("POST /monitors", srv.CreateMonitor)
	mux.HandleFunc("PUT /monitors/{id}", srv.UpdateMonitor)

	return http.ListenAndServe(":8080", mux)
}

func main() {
	if err := runServer(); err != nil {
		log.Fatal("Unable to create connection pool: ", err)
	}
}
