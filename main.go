package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5"
)

type Account struct {
	Id           int    `json:"id"`
	Name         string `json:"name"`
	Balance      int    `json:"balance"`
	BalanceLimit int    `json:"balance_limit"`
	CreatedAt    string `json:"created_at"`
}

func seedDB() {
	conn, err := pgx.Connect(context.Background(), "postgres://admin:123@localhost:5433/test-db")
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
	Valor int `json:"valor"`
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
	valor := reqBodyDTO.Valor

	conn, err := pgx.Connect(context.Background(), "postgres://admin:123@localhost:5433/test-db")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer conn.Close(context.Background())

	row := conn.QueryRow(context.Background(), "UPDATE accounts SET balance = balance + $1 WHERE id = $2 RETURNING balance, balance_limit", valor, id)
	var account Account
	err = row.Scan(&account.Balance, &account.BalanceLimit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to update balance: %v\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	responseBody := TransactionResponseBody{Saldo: account.Balance, Limite: account.BalanceLimit}
	w.WriteHeader(http.StatusOK)
	b, _ := json.Marshal(responseBody)
	w.Write(b)
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
