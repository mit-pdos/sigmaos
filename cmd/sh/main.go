package main

import (
	"log"
	"os"
	"strings"

	"ulambda/fs"
	"ulambda/proc"
)

func main() {
	log.Printf("Running: %v\n", os.Args[0])
	clnt := fs.InitFsClient(fs.MakeFsRoot(), os.Args[1:])
	for {
		b := []byte("λ ")
		_, err := clnt.Write(1, b)
		if err != nil {
			log.Fatal("Write error:", err)
		}
		b, err = clnt.Read(1, 1024) // XXX
		if err != nil {
			log.Fatal("Read error:", err)
		}
		cmd := strings.TrimSuffix(string(b), "\n")
		proc.Spawn(cmd, clnt)
	}
}
