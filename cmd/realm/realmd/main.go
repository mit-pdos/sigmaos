package main

import (
	"log"
	"os"

	"ulambda/realm"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %v bin id", os.Args[0])
	}

	r := realm.MakeRealmd(os.Args[1], os.Args[2])
	r.Work()
}
