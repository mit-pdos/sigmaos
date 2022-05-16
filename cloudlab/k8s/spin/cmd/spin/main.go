package main

import (
	"log"
)

const (
	PRINT_INTERVAL = 1_000_000_000
)

func main() {
	i := 0
	for {
		if i%PRINT_INTERVAL == 0 {
			log.Printf("i = %v", i)
		}
		i++
	}
}
