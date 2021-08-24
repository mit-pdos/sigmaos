package main

import (
	"log"
	"os"

	"ulambda/nps3"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("NPS3 incorrect number of args")
	}
	nps3 := nps3.MakeNps3(os.Args[1])
	nps3.Serve()
}
