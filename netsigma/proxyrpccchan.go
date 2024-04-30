package netsigma

import (
	"fmt"
	"net"
	"os"

	"golang.org/x/sys/unix"

	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/rpc"
	// "sigmaos/sessp"
)

var hasBeenInit bool

const (
	SIGMA_NETPROXY_FD = "SIGMA_NETPROXY_FD"
)

type NetProxyRPCCh struct {
	dmxclnt *demux.DemuxClnt
}

func NewNetProxyRPCCh(dmxclnt *demux.DemuxClnt) (*NetProxyRPCCh, error) {
	return &NetProxyRPCCh{
		dmxclnt: dmxclnt,
	}, nil
}

// Send an RPC request to either Dial or Listen
//func (ch *NetProxyRPCCh) SendReceive(iniov sessp.IoVec, outiov sessp.IoVec) error {
//	c := NewProxyCall(sessp.NextSeqno(scc.seqcntr), iniov)
//	rep, err := scc.dmx.SendReceive(c, outiov)
//	if err != nil {
//		return err
//	}
//	return nil
//}

func constructSocketControlMsg(proxiedFile *os.File) []byte {
	fd := int(proxiedFile.Fd())
	return unix.UnixRights(fd)
}

func parseReturnedConn(oob []byte) (*net.TCPConn, error) {
	// XXX sanity check
	if len(oob) == 0 {
		db.DPrintf(db.ERROR, "Error oob len 0")
		db.DFatalf("Error oob len 0")
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
	return fdToTCPConn(fds[0])
}

func fdToUnixConn(fd int) (*net.UnixConn, error) {
	// Create a FileConn from the file descriptor
	conn, err := fdToConn(fd)
	if err != nil {
		db.DFatalf("Error make FileConn: %v", err)
	}
	return conn.(*net.UnixConn), nil
}

func fdToTCPConn(fd int) (*net.TCPConn, error) {
	// Create a FileConn from the file descriptor
	conn, err := fdToConn(fd)
	if err != nil {
		db.DFatalf("Error make FileConn: %v", err)
	}
	return conn.(*net.TCPConn), nil
}

func fdToConn(fd int) (net.Conn, error) {
	// Make the  FD into a Golang file object
	f := os.NewFile(uintptr(fd), "netproxy-conn")
	if f == nil {
		db.DFatalf("Error new file")
	}
	// Create a FileConn from the file
	conn, err := net.FileConn(f)
	if err != nil {
		db.DFatalf("Error make FileConn (%v): %v", fd, err)
	}
	return conn, nil
}

//func fdToListener(fd int) (*net.TCPListener, error) {
//	f := os.NewFile(uintptr(fd), "tcp-listener")
//	if f == nil {
//		db.DFatalf("Error new file")
//	}
//	l, err := net.FileListener(f)
//	if err != nil {
//		db.DFatalf("Error make FileConn: %v", err)
//	}
//	return l.(*net.TCPListener), nil
//}

func (ch *NetProxyRPCCh) StatsSrv() (*rpc.RPCStatsSnapshot, error) {
	db.DPrintf(db.ERROR, "StatsSrv unimplemented")
	return nil, fmt.Errorf("Unimplemented")
}

//func (ch *NetProxyRPCCh) getReturnedFD() (int, error) {
//	b := make([]byte, len("Hello!"))
//	oob := make([]byte, unix.CmsgSpace(4))
//	// Send connection FD to child via socket
//	_, _, _, _, err := ch.conn.ReadMsgUnix(b, oob)
//	if err != nil {
//		db.DPrintf(db.NETPROXYCLNT_ERR, "Error recv proxied conn fd: err %v", err)
//		return 0, err
//	}
//	db.DPrintf(db.ALWAYS, "returned fd msg %s", b)
//	scma, err := unix.ParseSocketControlMessage(oob)
//	if err != nil {
//		db.DFatalf("Error parse socket control message: %v", err)
//	}
//	fds, err := unix.ParseUnixRights(&scma[0])
//	if err != nil || len(fds) != 1 {
//		db.DFatalf("Error parse unix rights: len %v err %v", len(fds), err)
//	}
//	db.DPrintf(db.NETPROXYCLNT, "got socket fd %v", fds[0])
//	return fds[0], nil
//}

// Receive the connection FD corresponding to a successful Dial request
//func (ch *NetProxyRPCCh) GetReturnedConn() (*net.TCPConn, error) {
//	fd, err := ch.getReturnedFD()
//	if err != nil {
//		return nil, err
//	}
//	return fdToTCPConn(fd)
//}

//// Receive the connection FD corresponding to a successful Listen request
//func (ch *NetProxyRPCCh) GetReturnedListener() (net.Listener, error) {
//	fd, err := ch.getReturnedFD()
//	if err != nil {
//		return nil, err
//	}
//	return fdToListener(fd)
//}
