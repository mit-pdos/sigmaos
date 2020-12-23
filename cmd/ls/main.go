package main

import (
	"log"
	"os"

	"ulambda/fs"
)

func main() {
	log.Printf("Running: %v\n", os.Args[0])
	clnt := fs.InitFsClient(fs.MakeFsRoot(), os.Args[1:])
	fd, err := clnt.Open("/")
	if err != nil {
		log.Fatal("Open error:", err)
	}
	if buf, err := clnt.Read(fd, 1024); err == nil {
		_, err := clnt.Write(1, buf)
		if err != nil {
			log.Fatal("Write error:", err)
		}
	} else {
		log.Fatal("Read error:", err)
	}
}
