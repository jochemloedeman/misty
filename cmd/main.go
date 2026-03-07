package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jochemloedeman/misty/db/sqlc"
	"github.com/jochemloedeman/misty/monitor/postgres"
	"github.com/jochemloedeman/misty/server"
	"github.com/jochemloedeman/misty/users"
	userdb "github.com/jochemloedeman/misty/users/postgres"
)

func runServer() error {
	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		return fmt.Errorf("unable to create connection pool: %w", err)
	}
	defer pool.Close()

	queries := sqlc.New(pool)

	if err := seedDevUser(context.Background(), queries); err != nil {
		return fmt.Errorf("failed to seed dev user: %w", err)
	}

	srv := server.New(postgres.NewMonitorStore(queries))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /monitors", srv.ListMonitors)
	mux.HandleFunc("GET /monitors/{id}", srv.GetMonitor)
	mux.HandleFunc("POST /monitors", srv.CreateMonitor)
	mux.HandleFunc("POST /monitors/{id}/deactivate", srv.SetMonitorStatus(false))
	mux.HandleFunc("POST /monitors/{id}/activate", srv.SetMonitorStatus(true))
	mux.HandleFunc("DELETE /monitors/{id}", srv.DeleteMonitor)

	return http.ListenAndServe(":8080", srv.RequireUser(mux))
}

func seedDevUser(ctx context.Context, q *sqlc.Queries) error {
	devUser := users.User{ID: uuid.MustParse("00000000-0000-0000-0000-000000000001")}
	userStore := userdb.NewUserStore(q)
	return userStore.Ensure(ctx, devUser)
}

func main() {
	if err := runServer(); err != nil {
		log.Fatal(err)
	}
}
