package main

import (
	"context"
	"database/sql"
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
	"github.com/jackc/pgx/v5/pgxpool"
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
	AccountId   int                `json:"account_id"`
	Amount      int                `json:"amount"`
	Type        string             `json:"type"`
	Description string             `json:"description"`
	CreatedAt   pgtype.Timestamptz `json:"created_at"`
}

var (
	ErrInsufficientFunds = errors.New("account does not have available limit for this debit amount")
	ErrNotFound          = errors.New("account not found")
	ConnPool             *pgxpool.Pool // shouldn't be global, better to use dependency injection. However, decided to do this way for this challenge.
)

func seedDB(pool *pgxpool.Pool) {
	seedSql, err := os.ReadFile("seed.sql")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error while trying to read seed.sql: %v\n", err)
		os.Exit(1)
	}

	_, err = pool.Exec(context.Background(), string(seedSql))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to seed database: %v\n", err)
		os.Exit(1)
	}
}

func connectDB(databaseURL string) *pgxpool.Pool {
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create connection pool to database: %v\n", err)
		os.Exit(1)
	}
	return pool
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
	accountId := r.PathValue("id")
	fmt.Printf("Making transaction for client of id %s...\n", accountId)

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

	// validations
	if amount <= 0 {
		fmt.Fprintf(os.Stderr, "Amount needs to be a positive integer\n")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if len(description) == 0 || len(description) > 10 {
		fmt.Fprintf(os.Stderr, "Description needs to have length between 1 and 10\n")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// wrap queries in a database transaction
	err = pgx.BeginFunc(context.Background(), ConnPool, func(tx pgx.Tx) error {
		// update account's balance
		var account Account

		switch transactionType {
		case "c":
			account, err = executeCredit(amount, accountId, tx)
		case "d":
			account, err = executeDebit(amount, accountId, tx)
			if err == ErrInsufficientFunds {
				w.WriteHeader(http.StatusUnprocessableEntity)
				return err
			}
		default:
			fmt.Fprint(os.Stderr, "Unknown bank transaction type\n")
			w.WriteHeader(http.StatusBadRequest)
			return err
		}

		if err == ErrNotFound {
			w.WriteHeader(http.StatusNotFound)
			return err
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to update balance: %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return err
		}

		// insert bank transaction
		_, err = tx.Exec(context.Background(), "INSERT INTO transactions (account_id, amount, type,  description) VALUES ($1, $2, $3, $4);", accountId, amount, transactionType, description)
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

func executeCredit(amount int, accountId string, tx pgx.Tx) (Account, error) {
	var account Account
	row := tx.QueryRow(context.Background(), "UPDATE accounts SET balance = balance + $1 WHERE id = $2 RETURNING balance, balance_limit;", amount, accountId)
	err := row.Scan(&account.Balance, &account.BalanceLimit)
	if errors.Is(err, pgx.ErrNoRows) {
		return account, ErrNotFound
	}
	return account, err
}

func executeDebit(amount int, accountId string, tx pgx.Tx) (Account, error) {
	var currAccount Account
	row := tx.QueryRow(context.Background(), "SELECT balance, balance_limit FROM accounts WHERE id = $1 FOR UPDATE;", accountId)
	err := row.Scan(&currAccount.Balance, &currAccount.BalanceLimit)
	if errors.Is(err, pgx.ErrNoRows) {
		return currAccount, ErrNotFound
	}
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
	row = tx.QueryRow(context.Background(), "UPDATE accounts SET balance = balance - $1 WHERE id = $2 RETURNING balance, balance_limit;", amount, accountId)
	err = row.Scan(&account.Balance, &account.BalanceLimit)
	return account, err
}

type Saldo struct {
	Total       int    `json:"total"`
	DataExtrato string `json:"data_extrato"`
	Limite      int    `json:"limite"`
}

type ActivityStatementTransaction struct {
	Valor       int    `json:"valor"`
	Tipo        string `json:"tipo"`
	Descricao   string `json:"descricao"`
	RealizadaEm string `json:"realizada_em"`
}

// had to create this after changing the query fetch accounts with transactions to LEFT JOIN
// the Scan method raises the following error: Unable to query transactions: can't scan into dest[2]: cannot scan NULL into *int
// using sql nullable values
type TransactionDBModel struct {
	Id          sql.NullString `json:"id"`
	AccountId   sql.NullInt64  `json:"account_id"`
	Amount      sql.NullInt64  `json:"amount"`
	Type        sql.NullString `json:"type"`
	Description sql.NullString `json:"description"`
	CreatedAt   sql.NullTime   `json:"created_at"`
}

type ActivityStatementResponseBody struct {
	Saldo             Saldo                          `json:"saldo"`
	UltimasTransacoes []ActivityStatementTransaction `json:"ultimas_transacoes"`
}

func activityStatementHandler(w http.ResponseWriter, r *http.Request) {
	accountId := r.PathValue("id")
	fmt.Printf("Reading activity statement of client with id %s...\n", accountId)

	rows, err := ConnPool.Query(context.Background(), `
    SELECT a.balance, a.balance_limit, t.amount, t.type, t.description, t.created_at
    FROM accounts a
    LEFT JOIN LATERAL (
      SELECT * FROM transactions t
      WHERE t.account_id = $1
      ORDER BY t.created_at DESC
      LIMIT 10
    ) t ON true
    WHERE a.id = $1;`, accountId)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to query transactions: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var account Account
	lastTransactions := []ActivityStatementTransaction{}

	hasNextRow := rows.Next()
	if !hasNextRow {
		fmt.Fprintf(os.Stderr, "Account not found\n")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	for hasNextRow {
		var transaction TransactionDBModel
		err = rows.Scan(&account.Balance, &account.BalanceLimit, &transaction.Amount, &transaction.Type, &transaction.Description, &transaction.CreatedAt)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Unable to query transactions: %v\n", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if transaction.Amount.Valid {
			activityStatementTransaction := ActivityStatementTransaction{Valor: int(transaction.Amount.Int64), Tipo: transaction.Type.String, Descricao: transaction.Description.String, RealizadaEm: transaction.CreatedAt.Time.UTC().Format(time.RFC3339)}
			lastTransactions = append(lastTransactions, activityStatementTransaction)
		}

		hasNextRow = rows.Next()
	}

	responseBody := ActivityStatementResponseBody{
		Saldo:             Saldo{Total: account.Balance, Limite: account.BalanceLimit, DataExtrato: time.Now().UTC().Format(time.RFC3339)},
		UltimasTransacoes: lastTransactions,
	}

	b, _ := json.Marshal(responseBody)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(b)
}

func getEnv(key, fallback string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		value = fallback
	}
	return value
}

func main() {
	fmt.Println("Starting up server...")

	PORT := getEnv("PORT", "9999")
	DB_HOSTNAME := getEnv("DB_HOSTNAME", "localhost")
	DB_USER := getEnv("DB_USER", "admin")
	DB_PASS := getEnv("DB_PASS", "123")
	DB_PORT := getEnv("DB_PORT", "5432")
	DB_NAME := getEnv("DB_NAME", "rinha-db")

	ConnPool = connectDB("postgres://" + DB_USER + ":" + DB_PASS + "@" + DB_HOSTNAME + ":" + DB_PORT + "/" + DB_NAME) // sets global pool variable
	// uncomment the seed below if wants to run it locally with go run main.go
	// seedDB(ConnPool)

	http.HandleFunc("GET /health", healthHandler)
	http.HandleFunc("POST /clientes/{id}/transacoes", transactionHandler)
	http.HandleFunc("GET /clientes/{id}/extrato", activityStatementHandler)

	fmt.Println("Listening to requests on port " + PORT)
	log.Fatal(http.ListenAndServe(":"+PORT, nil))
}
