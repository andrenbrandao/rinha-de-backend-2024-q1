package main

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestMain(t *testing.T) {
	conn, err := pgx.Connect(context.Background(), "postgres://admin:123@localhost:5433/test-db")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	conn.Exec(context.Background(), "DROP TABLE IF EXISTS accounts;")
	defer conn.Close(context.Background())

	t.Run("seeds the database with 5 accounts", func(t *testing.T) {
		seedDB()
		var got int
		conn.QueryRow(context.Background(), "SELECT COUNT(*) FROM accounts").Scan(&got)
		want := 5

		if got != want {
			t.Errorf("Got %d accounts, wants %d", got, want)
		}
	})
}
