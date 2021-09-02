package main

import (
	"log"

	"ulambda/kernel"
)

func main() {
	s := kernel.MakeSystem(".")
	err := s.Boot()
	if err != nil {
		log.Fatalf("Boot error: %v", err)
	}
}
