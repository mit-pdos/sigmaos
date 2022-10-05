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

	path := os.Getenv("REQ_PATH")
	if path == "" {
		log.Fatalf("No REQ_PATH supplied.")
	}

	query := os.Getenv("REQ_QUERY")
	if path == "mm" {
		if query == "" {
			log.Fatalf("No REQ_QUERY for path mm")
		}
	}

	resp, err := http.Get(url + "/" + path + query)
	if err != nil {
		log.Fatalf("Error GET: %v", err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error ReadAll: %v", err)
	}

	log.Printf("Got response: \"%v\"", strings.TrimSpace(string(body)))
}
