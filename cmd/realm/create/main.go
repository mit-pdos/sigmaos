package main

import (
	"log"
	"os"

	"ulambda/realm"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %v realmId", os.Args[0])
	}
	realm.MakeRealm(os.Args[1])
}
