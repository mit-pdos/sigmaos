package main

import (
	"log"
	"os"
	"strings"

	"ulambda/fsclnt"
	"ulambda/proc"
)

func main() {
	log.Printf("Running: %v\n", os.Args)
	// clnt, _, err := fsclnt.InitFsClient(fsclnt.MakeFsRoot(), os.Args[1:])
	clnt, _, err := fsclnt.InitFsClient(os.Args[1:])
	if err != nil {
		log.Fatal("InitFsClient error:", err)
	}
	for {
		b := []byte("λ ")
		_, err := clnt.Write(fsclnt.Stdout, 0, b)
		if err != nil {
			log.Fatal("Write error:", err)
		}
		b, err = clnt.Read(fsclnt.Stdin, 0, 1024)
		if err != nil {
			log.Fatal("Read error:", err)
		}
		line := strings.TrimSuffix(string(b), "\n")
		cmd := strings.Split(line, " ")
		child, err := proc.Spawn(clnt, cmd[0], cmd[1:], clnt.Lsof())
		if err == nil {
			status, err := proc.Wait(clnt, child)
			if err != nil {
				log.Fatal("Wait error:", err)
			}
			log.Printf("Wait: %v\n", string(status))
		}
	}
	proc.Exit(clnt, "OK")
}
