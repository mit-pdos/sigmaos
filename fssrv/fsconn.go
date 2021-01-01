package fssrv

import (
	"errors"
	"log"

	"net"

	np "ulambda/ninep"
)

type FsConn struct {
	fs   Fs
	conn net.Conn
	Fids map[np.Tfid]interface{}
}

func MakeFsConn(fs Fs, conn net.Conn) *FsConn {
	c := &FsConn{fs, conn, make(map[np.Tfid]interface{})}
	return c
}

func (conn *FsConn) Version(args np.Tversion, reply *np.Rversion) error {
	log.Printf("Version %v\n", args)
	v, ok := conn.fs.(VersionFs)
	if ok {
		return v.Version(conn, args, reply)
	} else {
		return errors.New("Not supported")
	}
}

func (conn *FsConn) Auth(args np.Tauth, reply *np.Rauth) error {
	log.Printf("Auth %v\n", args)
	v, ok := conn.fs.(AuthFs)
	if ok {
		return v.Auth(conn, args, reply)
	} else {
		return errors.New("Not supported")
	}
}

func (conn *FsConn) Attach(args np.Tattach, reply *np.Rattach) error {
	log.Printf("Attach %v from %v\n", args, conn.conn.RemoteAddr())
	v, ok := conn.fs.(AttachFs)
	if ok {
		return v.Attach(conn, args, reply)
	} else {
		return errors.New("Not supported")
	}
}

func (conn *FsConn) Flush(args np.Tflush, reply *np.Rflush) error {
	log.Printf("Attach %v\n", args)
	v, ok := conn.fs.(FlushFs)
	if ok {
		return v.Flush(conn, args, reply)
	} else {
		return errors.New("Not supported")
	}
}
