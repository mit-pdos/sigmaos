package netsigma

import (
	"fmt"
	"net"
	"os"

	"golang.org/x/sys/unix"

	db "sigmaos/debug"
	"sigmaos/frame"
	"sigmaos/rpc"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type NetProxyRPCCh struct {
	conn *net.UnixConn
}

func NewNetProxyRPCCh() (*NetProxyRPCCh, error) {
	// Connect to the netproxy server
	conn, err := net.Dial("unix", sp.SIGMA_NETPROXY_SOCKET)
	if err != nil {
		db.DPrintf(db.ERROR, "Error dial netproxy srv")
		return nil, err
	}
	return &NetProxyRPCCh{
		conn: conn.(*net.UnixConn),
	}, nil
}

// Send an RPC request to either Dial or Listen
func (ch *NetProxyRPCCh) SendReceive(iniov sessp.IoVec, outiov sessp.IoVec) error {
	if err := frame.WriteFrames(ch.conn, iniov); err != nil {
		db.DPrintf(db.NETPROXYCLNT_ERR, "Error WriteFrames: %v", err)
		return err
	}
	n, err := frame.ReadNumOfFrames(ch.conn)
	if err != nil {
		db.DPrintf(db.NETPROXYCLNT_ERR, "Error ReadNumOfFrames: %v", err)
		return err
	}
	if uint32(len(outiov)) != n {
		db.DFatalf("NetProxyRPCChan mismatch between supplied destination nvec and incoming nvec: %v != %v", len(outiov), n)
	}
	db.DPrintf(db.NETPROXYCLNT, "[%p] Read n frames: %v", ch.conn, len(outiov))
	if err := frame.ReadNFramesInto(ch.conn, outiov); err != nil {
		db.DPrintf(db.NETPROXYCLNT_ERR, "Error ReadNFramesInto: %v", err)
		return err
	}
	db.DPrintf(db.NETPROXYCLNT, "Read n done: %v", len(outiov))
	return nil
}

func fdToConn(fd int) (*net.TCPConn, error) {
	// Make the  FD into a Golang file object
	f := os.NewFile(uintptr(fd), "tcp-conn")
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

func fdToListener(fd int) (*net.TCPListener, error) {
	f := os.NewFile(uintptr(fd), "tcp-listener")
	if f == nil {
		db.DFatalf("Error new file")
	}
	l, err := net.FileListener(f)
	if err != nil {
		db.DFatalf("Error make FileConn: %v", err)
	}
	return l.(*net.TCPListener), nil
}

func (ch *NetProxyRPCCh) StatsSrv() (*rpc.RPCStatsSnapshot, error) {
	db.DPrintf(db.ERROR, "StatsSrv unimplemented")
	return nil, fmt.Errorf("Unimplemented")
}

func (ch *NetProxyRPCCh) getReturnedFD() (int, error) {
	oob := make([]byte, unix.CmsgSpace(4))
	// Send connection FD to child via socket
	_, _, _, _, err := ch.conn.ReadMsgUnix(nil, oob)
	if err != nil {
		db.DPrintf(db.NETPROXYCLNT_ERR, "Error recv proxied conn fd: err %v", err)
		return 0, err
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
	return fds[0], nil
}

// Receive the connection FD corresponding to a successful Dial request
func (ch *NetProxyRPCCh) GetReturnedConn() (*net.TCPConn, error) {
	fd, err := ch.getReturnedFD()
	if err != nil {
		return nil, err
	}
	return fdToConn(fd)
}

// Receive the connection FD corresponding to a successful Listen request
func (ch *NetProxyRPCCh) GetReturnedListener() (net.Listener, error) {
	fd, err := ch.getReturnedFD()
	if err != nil {
		return nil, err
	}
	return fdToListener(fd)
}
