package main

import (
	"log"

	"ulambda/ulambd"
)

func main() {
	ld := ulambd.MakeLambd()
	ld.Run()
	log.Printf("lambd: finished\n")
}
