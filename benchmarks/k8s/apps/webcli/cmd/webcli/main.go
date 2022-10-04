package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

func main() {
	url := os.Getenv("WEBSRV_URL")
	if url == "" {
		log.Fatalf("No WEBSRV_URL supplied.")
	}
	resp, err := http.Get(url + "/hello")
	if err != nil {
		log.Fatalf("Error GET: %v", err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error ReadAll: %v", err)
	}

	log.Printf("Got response: \"%v\"", strings.TrimSpace(string(body)))
}
