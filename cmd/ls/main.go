package main

import (
	"log"
	"os"

	"ulambda/fs"
	"ulambda/proc"
)

func main() {
	clnt, args, err := fs.InitFsClient(fs.MakeFsRoot(), os.Args[1:])

	log.Printf("Running: %v\n", args)
	if err != nil {
		proc.Exit(clnt, "InitFsClient error:", err)
	}
	var fd int
	if len(args) == 1 {
		fd, err = clnt.Open("/")
	} else {
		fd, err = clnt.Open(args[1])
	}
	if err != nil {
		proc.Exit(clnt, "Open error", err)
	}
	defer clnt.Close(fd)
	if buf, err := clnt.Read(fd, 1024); err == nil {
		_, err := clnt.Write(fs.Stdout, buf)
		if err != nil {
			proc.Exit(clnt, "Write error: ", err)
		}
	} else {
		proc.Exit(clnt, "Read error: ", err)
	}
	proc.Exit(clnt, "OK")
}
