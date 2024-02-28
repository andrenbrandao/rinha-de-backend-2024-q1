package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	fmt.Println("Starting up server...")

	http.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "Server is running!\n")
	})

	http.HandleFunc("GET /clientes/{id}/transacoes", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		fmt.Fprintf(w, "Transacoes...\n")
		fmt.Fprintf(w, "Cliente %s...\n", id)
	})

	fmt.Println("Listening to requests on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
