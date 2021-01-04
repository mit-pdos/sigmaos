package main

import (
	"log"
	"os"

	"ulambda/fsclnt"
	np "ulambda/ninep"
	"ulambda/proc"
)

func main() {
	log.Printf("Running: %v\n", os.Args)
	// args := os.Args[2:]
	clnt, err := fsclnt.InitFsClient(os.Args[1:])
	if err != nil {
		proc.Exit(clnt, "InitFsClient error:", err)
	}
	fd, err := clnt.Open("name", np.OREAD)
	if err != nil {
		proc.Exit(clnt, "Open error", err)
	}
	defer clnt.Close(fd)
	if buf, err := clnt.Read(fd, 0, 1024); err == nil {
		_, err := clnt.Write(fsclnt.Stdout, 0, buf)
		if err != nil {
			proc.Exit(clnt, "Write error: ", err)
		}
	} else {
		proc.Exit(clnt, "Read error: ", err)
	}
	proc.Exit(clnt, "OK")
}
