package fs

import (
	"log"
	"net"
	"net/rpc"
	"ulambda/fsrpc"
)

type Fs interface {
	Walk(string) (*fsrpc.Ufd, error)
	Open(string) (fsrpc.Fd, error)
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

func register(fs Fs, root bool) *fsrpc.Ufd {
	s := &FsSrv{fs}
	var l net.Listener
	var err error
	if root {
		l, err = net.Listen("tcp", ":1111")
	} else {
		l, err = net.Listen("tcp", ":0")
	}
	if err != nil {
		log.Fatal("Listen error:", err)
	}
	addr := l.Addr()
	log.Printf("addr %v\n", addr)
	fd := &fsrpc.Ufd{addr.String(), 0}
	rpc.Register(s)
	go runsrv(l)
	return fd
}
