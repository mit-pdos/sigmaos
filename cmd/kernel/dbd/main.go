package main

import (
	"fmt"
	"log"
	"os"

	"ulambda/dbd"
	"ulambda/fsclnt"
	"ulambda/named"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("Usage: dbd <pid>")
	}
	ip, err := fsclnt.LocalIP()
	if err != nil {
		log.Fatalf("LocalIP %v %v\n", named.UX, err)
	}
	dbd, err := dbd.MakeDbd(ip+":0", os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v: error %v", os.Args[0], err)
		os.Exit(1)
	}
	dbd.Serve()
}
