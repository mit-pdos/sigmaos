package netsrv

import (
	"bufio"
	"fmt"
	"io"
	"net"

	"runtime/debug"

	db "sigmaos/debug"
	"sigmaos/netsigma"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sessp"
	sps "sigmaos/sigmaprotsrv"
)

type WriteF func(*sessp.FcallMsg, []byte, *bufio.Writer) *serr.Err
type ReadF func(rdr io.Reader) (sessp.Tseqno, *sessp.FcallMsg, *serr.Err)

type NetServer struct {
	pcfg       *proc.ProcEnv
	addr       string
	sesssrv    sps.SessServer
	writefcall WriteF
	readframe  ReadF
	l          net.Listener
}

func NewNetServer(pcfg *proc.ProcEnv, ss sps.SessServer, address string, m WriteF, u ReadF) *NetServer {
	srv := &NetServer{pcfg: pcfg, sesssrv: ss, writefcall: m, readframe: u}

	// Create and start the main server listener
	var l net.Listener
	l, err := net.Listen("tcp", address)
	if err != nil {
		db.DFatalf("Listen error: %v", err)
	}
	a, err := netsigma.QualifyAddrLocalIP(pcfg.GetLocalIP(), l.Addr().String())
	if err != nil {
		db.DFatalf("QualifyAddr \"%v\" -> \"%v\" error: %v\n%s", l.Addr().String(), a, err, debug.Stack())
	}
	srv.addr = a
	srv.l = l
	db.DPrintf(db.PORT, "listen %v myaddr %v\n", address, a)
	go srv.runsrv(l)
	return srv
}

func (srv *NetServer) MyAddr() string {
	return srv.addr
}

func (srv *NetServer) CloseListener() error {
	db.DPrintf(db.ALWAYS, "Close %v\n", srv.addr)
	return srv.l.Close()
}

func (srv *NetServer) runsrv(l net.Listener) {
	for {
		conn, err := l.Accept()
		if err != nil {
			db.DPrintf(db.ALWAYS, "%v: Accept err %v", srv.pcfg.GetPID(), err)
			return
		}
		db.DPrintf(db.NETSRV, "accept %v %v\n", l, conn)
		NewSrvConn(srv, conn)
	}
}

func (srv *NetServer) String() string {
	return fmt.Sprintf("{ addr: %v }", srv.addr)
}
