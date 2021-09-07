package main

import (
	"log"
	"os"

	"ulambda/fsclnt"
	"ulambda/fsux"
	"ulambda/named"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalf("Usage: fsux <pid>")
	}
	ip, err := fsclnt.LocalIP()
	if err != nil {
		log.Fatalf("LocalIP %v %v\n", named.UX, err)
	}

	fsux := fsux.MakeFsUx("/tmp", ip+":0", os.Args[1])
	fsux.Serve()
}
