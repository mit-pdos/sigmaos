package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func hello(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "hello\n")
	log.Printf("hello!")
}

func main() {
	http.HandleFunc("/hello", hello)

	port := os.Getenv("PORT")
	if port == "" {
		log.Fatalf("No port")
	}
	http.ListenAndServe(port, nil)
}
