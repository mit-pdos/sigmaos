package main

import (
	"log"
	"os"

	"ulambda/fs"
)

func main() {
	log.Printf("Running: %v\n", os.Args[0])
	clnt := fs.InitFsClient(fs.MakeFsRoot(), os.Args[1:])
	for {
		b := []byte("λ ")
		_, err := clnt.Write(2, b) // XXX
		if err != nil {
			log.Fatal("Write error:", err)
		}
		b, err = clnt.Read(1, 1024) // XXX
		if err != nil {
			log.Fatal("Read error:", err)
		}
		log.Printf("read %v\n", b)
	}
}
