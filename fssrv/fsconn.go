package fssrv

import (
	"errors"
	"log"

	"net"

	np "ulambda/ninep"
)

type FsConn struct {
	fsd  Fsd
	conn net.Conn
	clnt FsClient
}

func MakeFsConn(fsd Fsd, conn net.Conn) *FsConn {
	clnt := fsd.Connect(conn)
	log.Printf("clnt %v\n", clnt)
	c := &FsConn{fsd, conn, clnt}
	return c
}

func (conn *FsConn) Version(args np.Tversion, reply *np.Rversion) error {
	log.Printf("Version %v\n", args)
	v, ok := conn.clnt.(VersionFs)
	if ok {
		return v.Version(args, reply)
	} else {
		return errors.New("Not supported")
	}
}

func (conn *FsConn) Auth(args np.Tauth, reply *np.Rauth) error {
	log.Printf("Auth %v\n", args)
	v, ok := conn.clnt.(AuthFs)
	if ok {
		return v.Auth(args, reply)
	} else {
		return errors.New("Not supported")
	}
}

func (conn *FsConn) Attach(args np.Tattach, reply *np.Rattach) error {
	log.Printf("Attach %v from %v\n", args, conn.conn.RemoteAddr())
	v, ok := conn.clnt.(AttachFs)
	if ok {
		return v.Attach(args, reply)
	} else {
		return errors.New("Not supported")
	}
}

func (conn *FsConn) Walk(args np.Twalk, reply *np.Rwalk) error {
	log.Printf("Walk %v from %v\n", args, conn.conn.RemoteAddr())
	v, ok := conn.clnt.(WalkFs)
	if ok {
		return v.Walk(args, reply)
	} else {
		return errors.New("Not supported")
	}
}

func (conn *FsConn) Create(args np.Tcreate, reply *np.Rcreate) error {
	log.Printf("Create %v from %v\n", args, conn.conn.RemoteAddr())
	v, ok := conn.clnt.(CreateFs)
	if ok {
		return v.Create(args, reply)
	} else {
		return errors.New("Not supported")
	}
}

func (conn *FsConn) Symlink(args np.Tsymlink, reply *np.Rsymlink) error {
	log.Printf("Symlink %v from %v\n", args, conn.conn.RemoteAddr())
	v, ok := conn.clnt.(SymlinkFs)
	if ok {
		return v.Symlink(args, reply)
	} else {
		return errors.New("Not supported")
	}
}

func (conn *FsConn) Readlink(args np.Treadlink, reply *np.Rreadlink) error {
	log.Printf("Readlink %v from %v\n", args, conn.conn.RemoteAddr())
	v, ok := conn.clnt.(ReadlinkFs)
	if ok {
		return v.Readlink(args, reply)
	} else {
		return errors.New("Not supported")
	}
}

func (conn *FsConn) Open(args np.Topen, reply *np.Ropen) error {
	log.Printf("Open %v from %v\n", args, conn.conn.RemoteAddr())
	v, ok := conn.clnt.(OpenFs)
	if ok {
		return v.Open(args, reply)
	} else {
		return errors.New("Not supported")
	}
}

func (conn *FsConn) Clunk(args np.Tclunk, reply *np.Rclunk) error {
	log.Printf("Clunk %v\n", args)
	v, ok := conn.clnt.(ClunkFs)
	if ok {
		return v.Clunk(args, reply)
	} else {
		return errors.New("Not supported")
	}
}

func (conn *FsConn) Flush(args np.Tflush, reply *np.Rflush) error {
	log.Printf("Flush %v\n", args)
	v, ok := conn.clnt.(FlushFs)
	if ok {
		return v.Flush(args, reply)
	} else {
		return errors.New("Not supported")
	}
}

func (conn *FsConn) Read(args np.Tread, reply *np.Rread) error {
	log.Printf("Read %v from %v\n", args, conn.conn.RemoteAddr())
	v, ok := conn.clnt.(ReadFs)
	if ok {
		return v.Read(args, reply)
	} else {
		return errors.New("Not supported")
	}
}

func (conn *FsConn) Write(args np.Twrite, reply *np.Rwrite) error {
	log.Printf("Write %v from %v\n", args, conn.conn.RemoteAddr())
	v, ok := conn.clnt.(WriteFs)
	if ok {
		return v.Write(args, reply)
	} else {
		return errors.New("Not supported")
	}
}
