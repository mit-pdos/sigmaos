package main

import (
	"errors"
	"log"
	"sync"

	"ulambda/fs"
	"ulambda/fsrpc"
	"ulambda/name"
)

type Named struct {
	mu   sync.Mutex
	done chan bool
	clnt *fs.FsClient
	srv  *name.Root
}

func makeNamed() *Named {
	nd := &Named{}
	nd.done = make(chan bool)
	return nd
}

func (nd *Named) Walk(path string) (*fsrpc.Ufd, string, error) {
	nd.mu.Lock()
	defer nd.mu.Unlock()
	ufd, rest, err := nd.srv.Walk(path)
	return ufd, rest, err
}

func (nd *Named) Open(ufd *fsrpc.Ufd) (fsrpc.Fd, error) {
	nd.mu.Lock()
	defer nd.mu.Unlock()

	inode, err := nd.srv.Open(ufd)
	fd := fsrpc.Fd(inode.Inum)
	return fd, err
}

func (nd *Named) Create(path string) (fsrpc.Fd, error) {
	return 0, errors.New("Unsupported")
}

func (nd *Named) Write(fd fsrpc.Fd, buf []byte) (int, error) {
	return 0, errors.New("Unsupported")
}

// XXX n
func (nd *Named) Read(fd fsrpc.Fd, n int) ([]byte, error) {
	nd.mu.Lock()
	defer nd.mu.Unlock()

	ls, err := nd.srv.Ls(fd, n)
	log.Printf("ls %v\n", ls)
	if err != nil {
		return nil, err
	}
	b := []byte(ls)
	return b, nil
}

func (nd *Named) Mount(fd *fsrpc.Ufd, path string) error {
	nd.mu.Lock()
	defer nd.mu.Unlock()

	return nd.srv.Mount(fd, path)
}

func main() {
	nd := makeNamed()
	nd.clnt, nd.srv = fs.MakeFs(nd, true)
	<-nd.done
}
