package main

import (
	"log"
	"os"

	"ulambda/realm"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %v bin", os.Args[0])
	}
	e := realm.MakeTestEnv(os.Args[1])
	_, err := e.Boot()
	if err != nil {
		log.Fatalf("Boot %v\n", err)
	}
}
