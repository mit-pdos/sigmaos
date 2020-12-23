package main

import (
	"log"

	"ulambda/fs"
	"ulambda/fsrpc"
)

type Named struct {
	names map[string]*fsrpc.Ufd
	done  chan bool
}

func (nd *Named) Walk(path string) (*fsrpc.Ufd, error) {
	log.Printf("Walk: %v\n", path)
	if fd, ok := nd.names[path]; ok {
		return fd, nil
	}
	return nil, nil
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

func (nd *Named) Read(fd fsrpc.Fd, n int) ([]byte, error) {
	return nil, nil
}

func (nd *Named) Mount(fd *fsrpc.Ufd, path string) error {
	log.Printf("Mount: %v\n", path)
	nd.names[path] = fd
	return nil
}

func main() {
	nd := &Named{make(map[string]*fsrpc.Ufd), make(chan bool)}
	clnt := fs.MakeFsClient(fs.MakeFsRoot())
	err := clnt.MkNod("/", nd)
	if err != nil {
		log.Fatal("Mknod error", err)
	}
	<-nd.done
}
