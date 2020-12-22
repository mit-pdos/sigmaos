package main

import (
	"log"

	"ulambda/fs"
)

func main() {
	clnt := fs.InitFsClient(os.Args)
	for {
		b := []byte("$\n")
		_, err := clnt.Write(1, b)
		if err != nil {
			log.Fatal("Write error:", err)
		}
		b, err = clnt.Read(0, 1024)
		if err != nil {
			log.Fatal("Read error:", err)
		}
		log.Printf("read %v\n", b)
	}
}
