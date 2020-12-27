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

func (nd *Named) Walk(start fsrpc.Fid, path string) (*fsrpc.Ufid, string, error) {
	nd.mu.Lock()
	defer nd.mu.Unlock()

	ufd, rest, err := nd.srv.Walk(start, path)
	return ufd, rest, err
}

func (nd *Named) Open(fid fsrpc.Fid, name string) (fsrpc.Fid, error) {
	nd.mu.Lock()
	defer nd.mu.Unlock()

	inode, err := nd.srv.Open(fid, name)
	if err != nil {
		return fsrpc.NullFid(), err
	}
	fd := fsrpc.Fid(inode.Fid)
	return fd, err
}

func (nd *Named) Create(fid fsrpc.Fid, name string) (fsrpc.Fid, error) {
	return fsrpc.NullFid(), errors.New("Unsupported")
}

func (nd *Named) Write(fd fsrpc.Fid, buf []byte) (int, error) {
	return 0, errors.New("Unsupported")
}

// XXX n
func (nd *Named) Read(fd fsrpc.Fid, n int) ([]byte, error) {
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

func (nd *Named) Mount(fd *fsrpc.Ufid, fid fsrpc.Fid, path string) error {
	nd.mu.Lock()
	defer nd.mu.Unlock()

	return nd.srv.Mount(fd, fid, path)
}

func main() {
	nd := makeNamed()
	nd.clnt, nd.srv = fs.MakeFs(nd, true)
	<-nd.done
}
