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
	clnt, err := fs.InitFsClient(fs.MakeFsRoot(), os.Args[1:])
	if err != nil {
		log.Fatal("InitFsClient error:", err)
	}
	for {
		b := []byte("λ ")
		_, err := clnt.Write(1, b)
		if err != nil {
			log.Fatal("Write error:", err)
		}
		b, err = clnt.Read(0, 1024)
		if err != nil {
			log.Fatal("Read error:", err)
		}
		cmd := strings.TrimSuffix(string(b), "\n")
		fd, err := proc.Spawn(clnt, cmd, clnt.Lsof())
		if err != nil {
			log.Fatal("Spawn error:", err)
		}
		err = proc.Wait(clnt, fd)
		if err != nil {
			log.Fatal("Wait error:", err)
		}
	}
}
