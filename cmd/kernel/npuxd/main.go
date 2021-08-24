package main

import (
	"log"
	"os"

	"ulambda/fsclnt"
	"ulambda/kernel"
	"ulambda/npux"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("Usage: npux <pid>")
	}
	ip, err := fsclnt.LocalIP()
	if err != nil {
		log.Fatalf("LocalIP %v %v\n", kernel.UX, err)
	}

	npux := npux.MakeNpUx("/tmp", ip+":0", os.Args[1])
	npux.Serve()
}
