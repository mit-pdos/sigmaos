package fs

import (
	"log"
	"net"
	"net/rpc"
	"ulambda/fsrpc"
)

type Fs interface {
	Walk(string) (*fsrpc.Ufd, string, error)
	Open(*fsrpc.Ufd) (fsrpc.Fd, error)
	Create(string) (fsrpc.Fd, error)
	Write(fsrpc.Fd, []byte) (int, error)
	Read(fsrpc.Fd, int) ([]byte, error)
	Mount(*fsrpc.Ufd, string) error
}

func runsrv(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal("Accept error: ", err)
		}
		go rpc.ServeConn(conn)
	}
}

func register(l net.Listener, fs Fs) {
	s := &FsSrv{fs}
	rpc.Register(s)
	go runsrv(l)
}
