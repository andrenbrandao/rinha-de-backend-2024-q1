package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
)

// You can use testing.T, if you want to test the code without benchmarking
func setupSuite(tb testing.TB) func(tb testing.TB) {
	tb.Log("Starting up test database...")

	cmd := exec.Command("docker", "compose", "-f", "docker-compose.tests.yml", "up", "-d")
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
		cmd := exec.Command("docker", "compose", "-f", "docker-compose.tests.yml", "down")
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
		err := row.Scan(&account.Id, &account.Name, &account.Balance, &account.BalanceLimit, &account.CreatedAt)
		if err != nil {
			t.Errorf("Unable to get account: %v\n", err)
			return
		}

		got := account.Balance
		want := 1500

		if got != want {
			t.Errorf("Got a balance of %d, wants %d", got, want)
		}
	})

	t.Run("POST /clientes/{id}/transacoes with credit type should return the new balance and current limit", func(t *testing.T) {
		seedDB()
		sendCreditRequestToAccount(1000, 2)
		res := sendCreditRequestToAccount(500, 2)

		var resBody TransactionResponseBody
		err := json.NewDecoder(res.Body).Decode(&resBody)
		if err != nil {
			t.Errorf("Unable to decode response body: %v\n", err)
			return
		}
		defer res.Body.Close()

		got := resBody.Saldo
		want := 1500

		if got != want {
			t.Errorf("Got a balance of %d, wants %d", got, want)
		}
	})

	t.Run("POST /clientes/{id}/transacoes inserts the transaction into the database", func(t *testing.T) {
		seedDB()
		sendCreditRequestToAccount(500, 2)

		row := conn.QueryRow(context.Background(), "SELECT * FROM transactions LIMIT 1;")
		var transaction Transaction
		err := row.Scan(&transaction.Id, &transaction.Amount, &transaction.Type, &transaction.Description, &transaction.CreatedAt)
		if err != nil {
			t.Errorf("Unable to get transaction: %v\n", err)
			return
		}

		got := transaction
		want := Transaction{Amount: 500, Type: "c", Description: "New description."}

		if got.Amount != want.Amount && got.Type != want.Type && got.Description != want.Description {
			t.Errorf("Got %v, want %v", got, want)
		}
	})

	t.Run("POST /clientes/{id}/transacoes with debit type should decrement the current balance", func(t *testing.T) {
		seedDB()
		sendDebitRequestToAccount(500, 2)

		row := conn.QueryRow(context.Background(), "SELECT * FROM accounts WHERE id = 2;")
		var account Account
		err := row.Scan(&account.Id, &account.Name, &account.Balance, &account.BalanceLimit, &account.CreatedAt)
		if err != nil {
			t.Errorf("Unable to get account: %v\n", err)
			return
		}

		got := account.Balance
		want := -500

		if got != want {
			t.Errorf("Got a balance of %d, wants %d", got, want)
		}
	})

	t.Run("POST /clientes/{id}/transacoes with debit type should not go over the balance limit", func(t *testing.T) {
		seedDB()
		res := sendDebitRequestToAccount(80000, 2)

		got := res.StatusCode
		want := http.StatusOK

		if got != want {
			t.Errorf("Got a status code of %d, wants %d", got, want)
		}

		res = sendDebitRequestToAccount(1, 2) // should not go over the limit

		got = res.StatusCode
		want = http.StatusUnprocessableEntity

		if got != want {
			t.Errorf("Got a status code of %d, wants %d", got, want)
		}

		row := conn.QueryRow(context.Background(), "SELECT * FROM accounts WHERE id = 2;")
		var account Account
		err := row.Scan(&account.Id, &account.Name, &account.Balance, &account.BalanceLimit, &account.CreatedAt)
		if err != nil {
			t.Errorf("Unable to get account: %v\n", err)
			return
		}

		got = account.Balance
		want = -80000

		if got != want {
			t.Errorf("Got a balance of %d, wants %d", got, want)
		}
	})

	t.Run("POST /clientes/{id}/transacoes concurrent requests should not let the balance go over the limit", func(t *testing.T) {
		seedDB()
		t.Setenv("IS_TEST_ENV", "true")

		var wg sync.WaitGroup
		wg.Add(2)
		go debitWorker(80000, 2, &wg)
		go debitWorker(80000, 2, &wg)
		wg.Wait()

		row := conn.QueryRow(context.Background(), "SELECT * FROM accounts WHERE id = 2;")
		var account Account
		err := row.Scan(&account.Id, &account.Name, &account.Balance, &account.BalanceLimit, &account.CreatedAt)
		if err != nil {
			t.Errorf("Unable to get account: %v\n", err)
			return
		}
		fmt.Println("Main test account balance", account.Balance)

		got := account.Balance
		want := -80000

		if got != want {
			t.Errorf("Got a balance of %d, wants %d", got, want)
		}
	})

	t.Run("POST /clientes/{id}/transacoes with unknown type should return bad request", func(t *testing.T) {
		seedDB()
		res := sendUnknownRequestToAccount(500, 2)

		got := res.StatusCode
		want := http.StatusBadRequest

		if got != want {
			t.Errorf("Got status %d, wants %d", got, want)
		}
	})
}

func sendCreditRequestToAccount(amount, id int) *http.Response {
	jsonStr := []byte(fmt.Sprintf(`{"valor": %d, "tipo": "c", "descricao": "New description"}`, amount))
	body := bytes.NewBuffer(jsonStr)
	req := httptest.NewRequest("POST", "/clientes/:id/transacoes", body)

	idStr := strconv.Itoa(id)
	req.SetPathValue("id", idStr)
	res := httptest.NewRecorder()
	transactionHandler(res, req)
	return res.Result()
}

func sendDebitRequestToAccount(amount, id int) *http.Response {
	jsonStr := []byte(fmt.Sprintf(`{"valor": %d, "tipo": "d", "descricao": "New description"}`, amount))
	body := bytes.NewBuffer(jsonStr)
	req := httptest.NewRequest("POST", "/clientes/:id/transacoes", body)

	idStr := strconv.Itoa(id)
	req.SetPathValue("id", idStr)
	res := httptest.NewRecorder()
	transactionHandler(res, req)
	return res.Result()
}

func debitWorker(amount int, id int, wg *sync.WaitGroup) {
	defer wg.Done()
	sendDebitRequestToAccount(amount, id)
}

func sendUnknownRequestToAccount(amount, id int) *http.Response {
	jsonStr := []byte(fmt.Sprintf(`{"valor": %d, "tipo": "u", "descricao": "New description"}`, amount))
	body := bytes.NewBuffer(jsonStr)
	req := httptest.NewRequest("POST", "/clientes/:id/transacoes", body)

	idStr := strconv.Itoa(id)
	req.SetPathValue("id", idStr)
	res := httptest.NewRecorder()
	transactionHandler(res, req)
	return res.Result()
}
