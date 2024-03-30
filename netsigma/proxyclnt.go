package netsigma

import (
	"net"
	"os"
	"sync"

	"golang.org/x/sys/unix"

	db "sigmaos/debug"
	"sigmaos/frame"
	"sigmaos/proc"
	//	"sigmaos/rpcclnt"
	sp "sigmaos/sigmap"
)

type NetProxyClnt struct {
	sync.Mutex
	init         bool
	proxyConn    *net.UnixConn
	pe           *proc.ProcEnv
	directDialFn DialFn
	proxyDialFn  DialFn
}

func NewNetProxyClnt(pe *proc.ProcEnv) *NetProxyClnt {
	return &NetProxyClnt{
		init:         false,
		pe:           pe,
		directDialFn: DialDirect,
	}
}

func (npc *NetProxyClnt) Dial(addr *sp.Taddr) (net.Conn, error) {
	var c net.Conn
	var err error
	// TODO XXX : a hack to force use by sigmaclntd procs. Remove GetPriv
	if npc.pe.GetUseSigmaclntd() || !npc.pe.GetPrivileged() {
		db.DPrintf(db.NETPROXYCLNT, "proxyDial %v", addr)
		c, err = npc.proxyDial(addr)
	} else {
		db.DPrintf(db.NETPROXYCLNT, "directDial %v", addr)
		c, err = npc.directDialFn(addr)
	}
	return c, err
}

func (npc *NetProxyClnt) initConnToNetProxySrv() error {
	npc.init = true
	// Connect to the netproxy server
	conn, err := net.Dial("unix", sp.SIGMA_NETPROXY_SOCKET)
	if err != nil {
		db.DPrintf(db.ERROR, "Error dial netproxy srv")
		return err
	}
	npc.proxyConn = conn.(*net.UnixConn)
	return nil
}

func (npc *NetProxyClnt) proxyDial(addr *sp.Taddr) (net.Conn, error) {
	npc.Lock()
	defer npc.Unlock()

	// Ensure that the connection to the netproxy server has been initialized
	if !npc.init {
		npc.initConnToNetProxySrv()
	}

	b := []byte(addr.Marshal())
	// Send the desired address to be dialed to the server
	frame.WriteFrame(npc.proxyConn, b)

	oob := make([]byte, unix.CmsgSpace(4))
	// Send connection FD to child via socket
	_, _, _, _, err := npc.proxyConn.ReadMsgUnix(nil, oob)
	if err != nil {
		db.DPrintf(db.NETPROXYCLNT_ERR, "Error recv proxied conn fd: err %v", err)
		return nil, err
	}
	scma, err := unix.ParseSocketControlMessage(oob)
	if err != nil {
		db.DFatalf("Error parse socket control message: %v", err)
	}
	fds, err := unix.ParseUnixRights(&scma[0])
	if err != nil || len(fds) != 1 {
		db.DFatalf("Error parse unix rights: len %v err %v", len(fds), err)
	}
	db.DPrintf(db.NETPROXYCLNT, "got socket fd %v", fds[0])
	// Make the returned FD into a Golang file object
	f := os.NewFile(uintptr(fds[0]), "tcp-conn")
	if f == nil {
		db.DFatalf("Error new file")
	}
	// Create a FileConn from the file
	pconn, err := net.FileConn(f)
	if err != nil {
		db.DFatalf("Error make FileConn: %v", err)
	}
	return pconn.(*net.TCPConn), nil
}
