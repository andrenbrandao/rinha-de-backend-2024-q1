package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/jackc/pgx/v5"
)

// You can use testing.T, if you want to test the code without benchmarking
func setupSuite(tb testing.TB) func(tb testing.TB) {
	tb.Log("Starting up test database...")

	cmd := exec.Command("docker", "compose", "up", "-d")
	err := cmd.Run()
	if err != nil {
		tb.Errorf("Unable to start test database: %v\n", err)
		os.Exit(1)
	}

	// wait for db to be ready
	tb.Log("Waiting for it to be ready...")
	cmd = exec.Command("docker", "run", "--rm", "--network", "rinha-de-backend-2024-q1_default", "busybox", "/bin/sh", "-c", "until nc -z test-db 5432; do sleep 3; echo 'Waiting for DB to come up...'; done")
	out, err := cmd.Output()
	fmt.Println(string(out))
	if err != nil {
		tb.Errorf("Error while waiting for DB to be ready: %v\n", err)
		fmt.Println(err.Error())
		os.Exit(1)
	}

	// Return a function to teardown the test
	return func(tb testing.TB) {
		tb.Log("Tearing down...")
		cmd = exec.Command("docker", "compose", "down")
		err = cmd.Run()
		if err != nil {
			tb.Errorf("Error while trying to shutdown database: %v\n", err)
			os.Exit(1)
		}
	}
}

func TestMain(t *testing.T) {
	tearDownSuite := setupSuite(t)
	defer tearDownSuite(t)

	conn, err := pgx.Connect(context.Background(), "postgres://admin:123@localhost:5433/test-db")
	if err != nil {
		t.Errorf("Unable to connect to test database: %v\n", err)
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
