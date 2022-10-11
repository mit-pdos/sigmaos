package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func init() {
	log.SetFlags(0)
}

func runCli(url, path, query string) {
	start := time.Now()
	resp, err := http.Get(url + "/" + path + query)
	if err != nil {
		log.Fatalf("Error GET: %v", err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error ReadAll: %v", err)
	}

	log.Printf("%v sec got response:\n\"%v\"", time.Since(start).Seconds(), strings.TrimSpace(string(body)))
}

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

	nclnt, err := strconv.Atoi(os.Getenv("N_CLNT"))
	if err != nil {
		log.Fatalf("Error Invalid N_CLINT (%v): %v", os.Getenv("N_CLNT"), err)
	}

	for i := 1; i < nclnt; i++ {
		done := make(chan bool)
		start := time.Now()
		for c := 0; c < i; c++ {
			go func() {
				runCli(url, path, query)
				done <- true
			}()
		}
		for c := 0; c < i; c++ {
			<-done
		}
		log.Printf("nclnt %d take %v(ms)\n", i, time.Since(start).Milliseconds())
	}
}
