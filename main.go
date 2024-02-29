package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5"
)

func seedDB() {
	conn, err := pgx.Connect(context.Background(), "postgres://admin:123@localhost:5433/test-db")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	_, err = conn.Exec(context.Background(), `CREATE TABLE IF NOT EXISTS accounts 
    (id SERIAL NOT NULL,
    name VARCHAR NOT NULL,
    balance INTEGER DEFAULT 0 NOT NULL,
    balance_limit INTEGER DEFAULT 0 NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW() NOT NULL)`)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create accounts table: %v\n", err)
		os.Exit(1)
	}

	_, err = conn.Exec(context.Background(), `
    INSERT INTO accounts
      (name, balance_limit)
    VALUES
      ('John Doe', 1000*100),
      ('Jane Doe', 800*100),
      ('Jack Sparrow', 10000*100),
      ('Bruce Wayne', 100000*100),
      ('Scarlett Johansson', 5000*100)
    `)

	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to insert seed data: %v\n", err)
		os.Exit(1)
	}
}

func main() {
	fmt.Println("Starting up server...")

	http.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "Server is running!\n")
	})

	http.HandleFunc("GET /clientes/{id}/extrato", func(w http.ResponseWriter, r *http.Request) {
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
	})

	fmt.Println("Listening to requests on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
