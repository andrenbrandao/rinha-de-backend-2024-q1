package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
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

	_, err = conn.Exec(context.Background(), "DROP TABLE IF EXISTS accounts;")
	if err != nil {
		t.Errorf("Unable to clean accounts table: %v\n", err)
	}
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

	t.Run("POST /clientes/{id}/transacoes with credit type should update the balance", func(t *testing.T) {
		seedDB()
		sendCreditRequestToAccount(1000, 2)
		sendCreditRequestToAccount(500, 2)

		row := conn.QueryRow(context.Background(), "SELECT * FROM accounts WHERE id = 2;")

		var account Account
		row.Scan(&account.Id, &account.Name, &account.Balance, &account.BalanceLimit, &account.CreatedAt)

		got := account.Balance
		want := 1500

		if got != want {
			t.Errorf("Got a balance of %d, wants %d", got, want)
		}
	})
}

func sendCreditRequestToAccount(amount, id int) {
	jsonStr := []byte(fmt.Sprintf(`{"valor": %d}`, amount))
	body := bytes.NewBuffer(jsonStr)
	req := httptest.NewRequest("POST", "/clientes/:id/transacoes", body)

	idStr := strconv.Itoa(id)
	req.SetPathValue("id", idStr)
	res := httptest.NewRecorder()
	transactionHandler(res, req)
}
