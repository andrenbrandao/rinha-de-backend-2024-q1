package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
)

func main() {
	fmt.Println("Starting up server...")

	http.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, "Server is running!\n")
	})

	fmt.Println("Listening to requests on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
