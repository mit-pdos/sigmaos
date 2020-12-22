package fs

import (
	"log"
	"net"
	"net/rpc"
	"ulambda/fsrpc"
)

type Fs interface {
	Open(string) (*fsrpc.Fd, error)
	Write([]byte) (int, error)
	Read(int) ([]byte, error)
	Mount(*fsrpc.Fd, string) error
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

func register(fs Fs, root bool) *fsrpc.Fd {
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
	fd := &fsrpc.Fd{addr.String()}
	rpc.Register(s)
	go runsrv(l)
	return fd
}
