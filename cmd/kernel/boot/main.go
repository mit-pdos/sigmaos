package main

import (
	"log"

	"ulambda/kernel"
)

func main() {
	_, err := kernel.Boot(".")
	if err != nil {
		log.Fatalf("Boot error: %v", err)
	}
}
