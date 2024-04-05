package netsigma

import (
	"fmt"
	"net"
	"os"
	"strconv"

	"golang.org/x/sys/unix"

	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/frame"
	"sigmaos/proc"
	"sigmaos/rpc"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

const (
	SIGMA_NETPROXY_FD = "SIGMA_NETPROXY_FD"
)

type NetProxyRPCCh struct {
	conn *net.UnixConn
}

func NewNetProxyRPCCh(pe *proc.ProcEnv) (*NetProxyRPCCh, error) {
	var conn *net.UnixConn
	fdstr := os.Getenv(SIGMA_NETPROXY_FD)
	if fdstr == "" {
		// Connect to the netproxy server by dialing the unix socket (should only
		// be done by the test program)
		uconn, err := net.Dial("unix", sp.SIGMA_NETPROXY_SOCKET)
		if err != nil {
			db.DPrintf(db.ERROR, "Error connect netproxy srv")
			return nil, err
		}
		conn = uconn.(*net.UnixConn)
	} else {
		// Connect to the netproxy server using the FD set up by the trampoline
		// (should be done by user procs)
		fd, err := strconv.Atoi(fdstr)
		if err != nil {
			db.DPrintf(db.ERROR, "Error get netproxy fd (%v): %v", fdstr, err)
			return nil, err
		}
		conn, err = fdToUnixConn(fd)
		if err != nil {
			db.DPrintf(db.ERROR, "Error connect netproxy srv")
			return nil, err
		}
	}
	b, err := proto.Marshal(pe.GetPrincipal())
	if err != nil {
		db.DFatalf("Error marshal principal: %v", err)
		return nil, err
	}
	// Write the authenticated principal ID to the server, so that the server
	// knows the principal associated with this connection
	if err := frame.WriteFrame(conn, b); err != nil {
		db.DPrintf(db.ERROR, "Error WriteFrame principal: %v", err)
		return nil, err
	}
	return &NetProxyRPCCh{
		conn: conn,
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
	f := os.NewFile(uintptr(fd), "tcp-conn")
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
	return fdToTCPConn(fd)
}

// Receive the connection FD corresponding to a successful Listen request
func (ch *NetProxyRPCCh) GetReturnedListener() (net.Listener, error) {
	fd, err := ch.getReturnedFD()
	if err != nil {
		return nil, err
	}
	return fdToListener(fd)
}
