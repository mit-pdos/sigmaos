package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %v <n> <sleep_length>\n", os.Args[0])
		os.Exit(1)
	}

	n, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "strconv err %v\n", err)
		os.Exit(1)
	}

	d, err := time.ParseDuration(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "ParseDuration err %v\n", err)
		os.Exit(1)
	}

	for i := 1; i < n; i++ {
		time.Sleep(d)
		f, err := os.OpenFile("log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			os.Exit(1)
		}
		fmt.Printf(".")
		if _, err := f.WriteString("Running..\n"); err != nil {
			os.Exit(1)
		}
		f.Close()
	}
}
