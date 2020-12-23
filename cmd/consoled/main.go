package main

import (
	"bufio"
	"log"
	"os"
	"ulambda/fs"
	"ulambda/fsrpc"
)

type Consoled struct {
	stdin  *bufio.Reader
	stdout *bufio.Writer
	done   chan bool
}

func (cons *Consoled) Walk(path string) (*fsrpc.Ufd, error) {
	return nil, nil
}

func (cons *Consoled) Open(path string) (fsrpc.Fd, error) {
	log.Printf("Open: %v\n", path)
	return 0, nil
}

func (cons *Consoled) Create(path string) (fsrpc.Fd, error) {
	return 0, nil
}

func (cons *Consoled) Write(fd fsrpc.Fd, buf []byte) (int, error) {
	n, err := cons.stdout.Write(buf)
	cons.stdout.Flush()
	return n, err
}

func (cons *Consoled) Read(fd fsrpc.Fd, n int) ([]byte, error) {
	b, err := cons.stdin.ReadBytes('\n')
	return b, err
}

func (cons *Consoled) Mount(fd *fsrpc.Ufd, path string) error {
	return nil
}

func main() {
	cons := &Consoled{
		bufio.NewReader(os.Stdin),
		bufio.NewWriter(os.Stdout),
		make(chan bool),
	}
	clnt := fs.MakeFsClient(fs.MakeFsRoot())
	err := clnt.MkNod("console", cons)
	if err != nil {
		log.Fatal("MkNod error:", err)
	}

	if fd, err := clnt.Open("console"); err == nil {
		err := clnt.Mount(fd, "/console")
		if err != nil {
			log.Fatal("Mount error:", err)
		}
	} else {
		log.Fatal("Open error:", err)
	}

	<-cons.done
	log.Printf("Consoled: finished %v\n", err)
}
