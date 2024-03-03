package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Account struct {
	Id           int                `json:"id"`
	Name         string             `json:"name"`
	Balance      int                `json:"balance"`
	BalanceLimit int                `json:"balance_limit"`
	CreatedAt    pgtype.Timestamptz `json:"created_at"`
}

type Transaction struct {
	Id          int                `json:"id"`
	Amount      int                `json:"amount"`
	Type        string             `json:"type"`
	Description string             `json:"description"`
	CreatedAt   pgtype.Timestamptz `json:"created_at"`
}

var (
	ErrInsufficientFunds = errors.New("account does not have available limit for this debit amount")
	PORT                 = "5433"
	DATABASE             = "test-db"
)

func seedDB() {
	conn, err := pgx.Connect(context.Background(), "postgres://admin:123@localhost:"+PORT+"/"+DATABASE)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	seedSql, err := os.ReadFile("seed.sql")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while trying to read seed.sql: %v\n", err)
		os.Exit(1)
	}

	_, err = conn.Exec(context.Background(), string(seedSql))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to seed database: %v\n", err)
		os.Exit(1)
	}
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprintf(w, "Server is running!\n")
}

type TransactionRequestBody struct {
	Valor     int    `json:"valor"`
	Tipo      string `json:"tipo"` // 'c' for credit and 'd' for debit
	Descricao string `json:"descricao"`
}

type TransactionResponseBody struct {
	Limite int `json:"limite"`
	Saldo  int `json:"saldo"`
}

func transactionHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	fmt.Printf("Executando transação do cliente de id %s...\n", id)

	reqBody, err := io.ReadAll(r.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot read request body: %v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var reqBodyDTO TransactionRequestBody
	err = json.Unmarshal(reqBody, &reqBodyDTO)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot parse request body: %v\n", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	amount := reqBodyDTO.Valor
	transactionType := reqBodyDTO.Tipo
	description := reqBodyDTO.Descricao

	conn, err := pgx.Connect(context.Background(), "postgres://admin:123@localhost:"+PORT+"/"+DATABASE)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer conn.Close(context.Background())

	// wrap queries in a database transaction
	err = pgx.BeginFunc(context.Background(), conn, func(tx pgx.Tx) error {
		// update account's balance
		var account Account

		switch transactionType {
		case "c":
			account, err = executeCredit(amount, id, tx)
		case "d":
			account, err = executeDebit(amount, id, tx)
			if err == ErrInsufficientFunds {
				w.WriteHeader(http.StatusUnprocessableEntity)
				return err
			}
		default:
			fmt.Fprint(os.Stderr, "Unknown bank transaction type\n")
			w.WriteHeader(http.StatusBadRequest)
			return err
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to update balance: %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return err
		}

		// insert bank transaction
		_, err = tx.Exec(context.Background(), "INSERT INTO transactions (amount, type,  description) VALUES ($1, $2, $3);", amount, transactionType, description)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to insert transaction: %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return err
		}

		// creates http response
		responseBody := TransactionResponseBody{Saldo: account.Balance, Limite: account.BalanceLimit}
		w.WriteHeader(http.StatusOK)
		b, _ := json.Marshal(responseBody)
		w.Write(b)
		return nil
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "DB transaction failed: %v\n", err)
		return
	}
}

func executeCredit(amount int, id string, tx pgx.Tx) (Account, error) {
	var account Account
	row := tx.QueryRow(context.Background(), "UPDATE accounts SET balance = balance + $1 WHERE id = $2 RETURNING balance, balance_limit;", amount, id)
	err := row.Scan(&account.Balance, &account.BalanceLimit)
	return account, err
}

func executeDebit(amount int, id string, tx pgx.Tx) (Account, error) {
	var currAccount Account
	row := tx.QueryRow(context.Background(), "SELECT balance, balance_limit FROM accounts WHERE id = $1 FOR UPDATE;", id)
	err := row.Scan(&currAccount.Balance, &currAccount.BalanceLimit)
	if err != nil {
		return currAccount, err
	}

	/*
		    HANDLING CONCURRENCY

			  - request A has to wait here until request B reaches this part to simulate the concurrency issue
			  - method A will run on the main test thread and will wait for the B execution
			  - method B will be a goroutine, and will wait on this part by using sleep
			  - after that, both will execute this second section and have read the same balance

	*/

	// wait for the other thread to reach
	// have to find a better way to test it
	if os.Getenv("IS_TEST_ENV") == "true" {
		time.Sleep(200 * time.Millisecond)
	}

	if currAccount.Balance-amount < -1*currAccount.BalanceLimit {
		return currAccount, ErrInsufficientFunds
	}

	var account Account
	row = tx.QueryRow(context.Background(), "UPDATE accounts SET balance = balance - $1 WHERE id = $2 RETURNING balance, balance_limit;", amount, id)
	err = row.Scan(&account.Balance, &account.BalanceLimit)
	return account, err
}

func extratoHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	fmt.Printf("Retornando extrato do cliente de id %s...\n", id)
	w.Header().Set("Content-Type", "application/json")

	responseBody := struct {
		Saldo struct {
			Total       int    `json:"total"`
			DataExtrato string `json:"data_extrato"`
			Limite      int    `json:"limite"`
		} `json:"saldo"`
	}{struct {
		Total       int    `json:"total"`
		DataExtrato string `json:"data_extrato"`
		Limite      int    `json:"limite"`
	}{Total: -9098, DataExtrato: "2024-01-17T02:34:41.217753Z", Limite: 100000}}

	b, _ := json.Marshal(responseBody)
	w.WriteHeader(http.StatusOK)
	w.Write(b)
}

func main() {
	fmt.Println("Starting up server...")
	seedDB()

	http.HandleFunc("GET /health", healthHandler)
	http.HandleFunc("POST /clientes/{id}/transacoes", transactionHandler)
	http.HandleFunc("GET /clientes/{id}/extrato", extratoHandler)

	fmt.Println("Listening to requests on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
