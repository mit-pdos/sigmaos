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

func (conn *FsConn) Walk(args np.Twalk, reply *np.Rwalk) error {
	log.Printf("Walk %v from %v\n", args, conn.conn.RemoteAddr())
	v, ok := conn.fs.(WalkFs)
	if ok {
		return v.Walk(conn, args, reply)
	} else {
		return errors.New("Not supported")
	}
}

func (conn *FsConn) Create(args np.Tcreate, reply *np.Rcreate) error {
	log.Printf("Create %v from %v\n", args, conn.conn.RemoteAddr())
	v, ok := conn.fs.(CreateFs)
	if ok {
		return v.Create(conn, args, reply)
	} else {
		return errors.New("Not supported")
	}
}

func (conn *FsConn) Clunk(args np.Tclunk, reply *np.Rclunk) error {
	log.Printf("Clunk %v\n", args)
	v, ok := conn.fs.(ClunkFs)
	if ok {
		return v.Clunk(conn, args, reply)
	} else {
		return errors.New("Not supported")
	}
}

func (conn *FsConn) Flush(args np.Tflush, reply *np.Rflush) error {
	log.Printf("Flush %v\n", args)
	v, ok := conn.fs.(FlushFs)
	if ok {
		return v.Flush(conn, args, reply)
	} else {
		return errors.New("Not supported")
	}
}
