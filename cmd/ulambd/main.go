package main

import (
	"log"

	"ulambda/ulambdad"
)

func main() {
	ld := ulambd.MakeLambd()
	ld.Scheduler()
	log.Printf("lambd: finished\n")
}
