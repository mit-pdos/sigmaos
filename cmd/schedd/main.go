package main

import (
	"log"

	"ulambda/schedd"
)

func main() {
	ld, err := schedd.MakeSchedd()
	if err == nil {
		ld.Scheduler()
	} else {
		log.Fatalf("schedd: error %v", err)
	}
}
