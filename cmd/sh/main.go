package main

import (
	"log"
	"os"
	"strings"

	"ulambda/fs"
	"ulambda/proc"
)

func main() {
	log.Printf("Running: %v\n", os.Args)
	clnt, _, err := fs.InitFsClient(fs.MakeFsRoot(), os.Args[1:])
	if err != nil {
		log.Fatal("InitFsClient error:", err)
	}
	for {
		b := []byte("λ ")
		_, err := clnt.Write(fs.Stdout, b)
		if err != nil {
			log.Fatal("Write error:", err)
		}
		b, err = clnt.Read(fs.Stdin, 1024)
		if err != nil {
			log.Fatal("Read error:", err)
		}
		line := strings.TrimSuffix(string(b), "\n")
		cmd := strings.Split(line, " ")
		child, err := proc.Spawn(clnt, cmd[0], cmd[1:], clnt.Lsof())
		if err != nil {
			log.Fatal("Spawn error:", err)
		}
		status, err := proc.Wait(clnt, child)
		if err != nil {
			log.Fatal("Wait error:", err)
		}
		log.Printf("Wait: %v\n", string(status))
	}
	proc.Exit(clnt, "OK")
}
