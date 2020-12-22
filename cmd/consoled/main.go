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

func (cons *Consoled) Open(path string) (*fsrpc.Fd, error) {
	return nil, nil
}

func (cons *Consoled) Write(buf []byte) (int, error) {
	n, err := cons.stdout.Write(buf)
	cons.stdout.Flush()
	return n, err
}

func (cons *Consoled) Read(n int) ([]byte, error) {
	b, err := cons.stdin.ReadBytes('\n')
	log.Printf("Read %v\n", b)
	return b, err
}

func (cons *Consoled) Mount(fd *fsrpc.Fd, path string) error {
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
