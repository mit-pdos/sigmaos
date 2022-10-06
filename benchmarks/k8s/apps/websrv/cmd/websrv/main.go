package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"gonum.org/v1/gonum/mat"
)

func hello(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "hello\n")
	log.Printf("hello!")
}

func mm(w http.ResponseWriter, req *http.Request) {
	start := time.Now()
	query := req.URL.Query()
	n, err := strconv.Atoi(query["n"][0])
	// Return an error if the requesr didn't contain a valid n
	if err != nil {
		http.Error(w, fmt.Sprintf("Error unmarshalling n: %v", err), http.StatusBadRequest)
		return
	}

	m1 := matrix(n)
	m2 := matrix(n)
	m3 := matrix(n)
	// Multiply m.m1 and m.m2, and place the result in m.m3
	m3.Mul(m1, m2)
	sec := time.Since(start).Seconds()
	fmt.Fprintf(w, "%v sec: %vx%v mm done!\n", sec, n, n)
	log.Printf("%v sec: %vx%v mm done!", sec, n, n)
}

// Create an n x n matrix.
func matrix(n int) *mat.Dense {
	s := make([]float64, n*n)
	for i := 0; i < n*n; i++ {
		s[i] = float64(i)
	}
	return mat.NewDense(n, n, s)
}

func main() {
	http.HandleFunc("/hello", hello)
	http.HandleFunc("/mm", mm)

	port := os.Getenv("PORT")
	if port == "" {
		log.Fatalf("No PORT supplied")
	}
	http.ListenAndServe(port, nil)
}
