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
