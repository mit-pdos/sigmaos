package fs

import (
	"log"
	"net"
	"net/rpc"
	"ulambda/fid"
)

type Fs interface {
	Walk(fid.Fid, string) (*fid.Ufid, string, error)
	Open(fid.Fid) error
	Create(fid.Fid, fid.IType, string) (fid.Fid, error)
	Remove(fid.Fid, string) error
	Symlink(fid.Fid, string, *fid.Ufid, string) error
	Pipe(fid.Fid, string) error
	Mount(*fid.Ufid, fid.Fid, string) error
	Write(fid.Fid, []byte) (int, error)
	Read(fid.Fid, int) ([]byte, error)
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
