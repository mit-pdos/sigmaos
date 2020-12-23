package main

import (
	"errors"
	"log"
	"strings"
	"sync"

	"ulambda/fs"
	"ulambda/fsrpc"
)

type Named struct {
	mu    sync.Mutex
	names map[string]*fsrpc.Ufd
	done  chan bool
}

func (nd *Named) Walk(path string) (*fsrpc.Ufd, error) {
	nd.mu.Lock()
	defer nd.mu.Unlock()

	log.Printf("Walk: %v\n", path)
	if path == "/" {
		// XXX my root
		return fs.MakeFsRoot(), nil
	}
	if fd, ok := nd.names[path]; ok {
		return fd, nil
	}
	return nil, errors.New("Read: unknown name")
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

	names := make([]string, 0, len(nd.names))
	for k, _ := range nd.names {
		names = append(names, k)
	}
	b := []byte(strings.Join(names, " ") + "/n")
	return b, nil
}

func (nd *Named) Mount(fd *fsrpc.Ufd, path string) error {
	nd.mu.Lock()
	defer nd.mu.Unlock()

	log.Printf("Mount: %v\n", path)
	nd.names[path] = fd
	return nil
}

func makeNamed() *Named {
	nd := &Named{}
	nd.names = make(map[string]*fsrpc.Ufd)
	nd.done = make(chan bool)
	return nd
}

func main() {
	nd := makeNamed()
	clnt := fs.MakeFsClient(fs.MakeFsRoot())
	err := clnt.MkNod("/", nd)
	if err != nil {
		log.Fatal("Mknod error", err)
	}
	<-nd.done
}
