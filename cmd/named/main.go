package main

import (
	"log"
	"sync"

	"ulambda/fs"
	"ulambda/fsrpc"
	"ulambda/name"
)

type Named struct {
	mu   sync.Mutex
	done chan bool
	root *name.Root
}

func makeNamed(root *fsrpc.Ufd) *Named {
	nd := &Named{}
	nd.root = name.MakeRoot(root)
	nd.done = make(chan bool)
	return nd
}

func (nd *Named) Walk(path string) (*fsrpc.Ufd, error) {
	nd.mu.Lock()
	defer nd.mu.Unlock()
	ufd, err := nd.root.Walk(path)
	log.Printf("Walk %v -> %v\n", path, ufd)
	return ufd, err
}

func (nd *Named) Open(path string) (fsrpc.Fd, error) {
	return 0, nil
}

func (nd *Named) Create(path string) (fsrpc.Fd, error) {
	return 0, nil
}

func (nd *Named) Write(fd fsrpc.Fd, buf []byte) (int, error) {
	return 0, nil
}

// XXX n
func (nd *Named) Read(fd fsrpc.Fd, n int) ([]byte, error) {
	nd.mu.Lock()
	defer nd.mu.Unlock()

	ls, err := nd.root.Ls(fd, n)
	if err != nil {
		return nil, err
	}
	b := []byte(ls)
	return b, nil
}

func (nd *Named) Mount(fd *fsrpc.Ufd, path string) error {
	nd.mu.Lock()
	defer nd.mu.Unlock()

	return nd.root.Mount(fd, path)
}

func main() {
	root := fs.MakeFsRoot()
	nd := makeNamed(root)
	clnt := fs.MakeFsClient(root)
	err := clnt.MkNod("/", nd)
	if err != nil {
		log.Fatal("Mknod error", err)
	}
	<-nd.done
}
