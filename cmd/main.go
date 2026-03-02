package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jochemloedeman/misty/internal/database"
)

func main() {
	conn, err := pgx.Connect(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	q := database.New(conn)
	params := database.CreateMonitorParams{
		IsActive:     true,
		LocationName: "test",
		Latitude:     50.,
		Longitude:    50.,
	}
	_, err = q.CreateMonitor(context.Background(), params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create monitor: %v\n", err)
		os.Exit(1)
	}

}
