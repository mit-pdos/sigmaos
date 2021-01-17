package main

import (
	"log"

	"ulambda/ulambdad"
)

func main() {
	ld := ulambd.MakeLambd()
	ld.Run()
	log.Printf("lambd: finished\n")
}
