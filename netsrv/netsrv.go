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
	sp "sigmaos/sigmap"
	sps "sigmaos/sigmaprotsrv"
)

type WriteF func(*sessp.FcallMsg, []byte, *bufio.Writer) *serr.Err
type ReadF func(rdr io.Reader) (sessp.Tseqno, *sessp.FcallMsg, *serr.Err)

type NetServer struct {
	pcfg       *proc.ProcEnv
	addr       *sp.Taddr
	sesssrv    sps.SessServer
	writefcall WriteF
	readframe  ReadF
	l          net.Listener
}

func NewNetServer(pcfg *proc.ProcEnv, ss sps.SessServer, addr *sp.Taddr, m WriteF, u ReadF) *NetServer {
	srv := &NetServer{pcfg: pcfg, sesssrv: ss, writefcall: m, readframe: u}

	db.DPrintf(db.PORT, "Listen addr %v", addr.IPPort())
	// Create and start the main server listener
	var l net.Listener
	l, err := net.Listen("tcp", addr.IPPort())
	if err != nil {
		db.DFatalf("Listen error: %v", err)
	}
	h, p, err := netsigma.QualifyAddrLocalIP(pcfg.GetInnerContainerIP(), l.Addr().String())
	if err != nil {
		db.DFatalf("QualifyAddr \"%v\" -> \"%v:%v\" error: %v\n%s", l.Addr().String(), h, p, err, debug.Stack())
	}
	srv.addr = sp.NewTaddrRealm(h, sp.INNER_CONTAINER_IP, p, pcfg.GetNet())
	srv.l = l
	db.DPrintf(db.PORT, "listen %v myaddr %v\n", addr, srv.addr)
	go srv.runsrv(l)
	return srv
}

func (srv *NetServer) MyAddr() *sp.Taddr {
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
