package main

import (
	"bufio"
	"errors"
	"log"
	"os"
	"sync"

	"ulambda/fs"
	"ulambda/fsrpc"
	"ulambda/name"
)

const (
	Stdin  fsrpc.Fd = fsrpc.Fd(name.RootInum) + 1
	Stdout fsrpc.Fd = fsrpc.Fd(name.RootInum) + 2
)

type Consoled struct {
	mu     sync.Mutex
	stdin  *bufio.Reader
	stdout *bufio.Writer
	clnt   *fs.FsClient
	srv    *name.Root
	done   chan bool
}

func makeConsoled() *Consoled {
	cons := &Consoled{}
	cons.stdin = bufio.NewReader(os.Stdin)
	cons.stdout = bufio.NewWriter(os.Stdout)
	cons.done = make(chan bool)
	return cons
}

func (cons *Consoled) Walk(path string) (*fsrpc.Ufd, string, error) {
	cons.mu.Lock()
	defer cons.mu.Unlock()

	ufd, rest, err := cons.srv.Walk(path)
	return ufd, rest, err
}

func (cons *Consoled) Open(ufd *fsrpc.Ufd) (fsrpc.Fd, error) {
	cons.mu.Lock()
	defer cons.mu.Unlock()

	inode, err := cons.srv.Open(ufd)
	if err != nil {
		return fsrpc.Fd(0), err
	}
	fd := fsrpc.Fd(inode.Inum)
	return fd, err
}

func (cons *Consoled) Create(path string) (fsrpc.Fd, error) {
	return 0, errors.New("Unsupported")
}

func (cons *Consoled) Write(fd fsrpc.Fd, buf []byte) (int, error) {
	if fd != Stdout {
		return 0, errors.New("Cannot write to this fd")
	}

	n, err := cons.stdout.Write(buf)
	cons.stdout.Flush()
	return n, err
}

func (cons *Consoled) Read(fd fsrpc.Fd, n int) ([]byte, error) {
	if fd != Stdin {
		return nil, errors.New("Cannot read from this fd")
	}

	b, err := cons.stdin.ReadBytes('\n')
	return b, err
}

func (cons *Consoled) Mount(fd *fsrpc.Ufd, path string) error {
	return errors.New("Unsupported")
}

func (cons *Consoled) FsInit() {
	err := cons.srv.Create("stdin", name.InodeNumber(Stdin), nil)
	if err != nil {
		log.Fatal("Create error: ", err)
	}
	err = cons.srv.Create("stdout", name.InodeNumber(Stdout), nil)
	if err != nil {
		log.Fatal("Create error: ", err)
	}
}

func main() {
	cons := makeConsoled()
	cons.clnt, cons.srv = fs.MakeFs(cons, false)
	cons.FsInit()
	if fd, err := cons.clnt.Open("."); err == nil {
		err := cons.clnt.Mount(fd, "/console")
		if err != nil {
			log.Fatal("Mount error:", err)
		}
	} else {
		log.Fatal("Open error:", err)
	}
	<-cons.done
	log.Printf("Consoled: finished\n")
}
