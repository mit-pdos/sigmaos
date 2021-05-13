package main

import (
	"log"

	"ulambda/fsclnt"
	"ulambda/fslib"
	"ulambda/npux"
)

func main() {
	ip, err := fsclnt.LocalIP()
	if err != nil {
		log.Fatalf("LocalIP %v %v\n", fslib.UX, err)
	}

	npux := npux.MakeNpUx("/tmp", ip+":0")
	npux.Serve()
}
