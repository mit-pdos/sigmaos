package fs

import (
	"log"
	"net"
	"net/rpc"
	"ulambda/fsrpc"
)

type Fs interface {
	Walk(fsrpc.Fid, string) (*fsrpc.Ufid, string, error)
	Open(fsrpc.Fid, string) (fsrpc.Fid, error)
	Create(fsrpc.Fid, string) (fsrpc.Fid, error)
	Write(fsrpc.Fid, []byte) (int, error)
	Read(fsrpc.Fid, int) ([]byte, error)
	Mount(*fsrpc.Ufid, fsrpc.Fid, string) error
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
