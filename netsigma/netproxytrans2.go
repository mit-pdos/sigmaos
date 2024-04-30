package netsigma

import (
	"fmt"
	"net"
	"os"
	"runtime/debug"
	"strconv"

	"golang.org/x/sys/unix"
	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/frame"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type NetProxyTrans struct {
	conn *net.UnixConn
	iovm *demux.IoVecMap
}

func NewNetProxyTrans(conn *net.UnixConn, iovm *demux.IoVecMap) *NetProxyTrans {
	return &NetProxyTrans{
		conn: conn,
		iovm: iovm,
	}
}

func getNetproxydConn(pe *proc.ProcEnv) (*net.UnixConn, error) {
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
		// Sanity check that a proc only has one NetProxyClnt, since using the fd
		// set up by the trampoline consumes it destructively.
		if hasBeenInit {
			db.DPrintf(db.ERROR, "Error double-init netproxyclnt")
			return nil, fmt.Errorf("Error double-init netproxyclnt: %v", string(debug.Stack()))
		}
		hasBeenInit = true
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
	return conn, nil
}

func (trans *NetProxyTrans) ReadCall() (demux.CallI, *serr.Err) {
	db.DPrintf(db.NETPROXYTRANS, "ReadCall trans conn [%p]", trans.conn)
	seqno, err := frame.ReadSeqno(trans.conn)
	if err != nil {
		db.DPrintf(db.NETPROXYTRANS_ERR, "Error ReadSeqno: %v", err)
		return nil, err
	}
	iov, _ := trans.iovm.Get(sessp.Ttag(seqno))
	if len(iov) == 0 {
		// Read frames, creating an IO vec
		iov, err = frame.ReadFrames(trans.conn)
		if err != nil {
			db.DPrintf(db.NETPROXYTRANS_ERR, "Error ReadFrames: %v", err)
			return nil, err
		}
	} else {
		n, err := frame.ReadNumOfFrames(trans.conn)
		if err != nil {
			db.DPrintf(db.NETPROXYTRANS_ERR, "Error ReadNumOfFrames: %v", err)
			return nil, err
		}
		if uint32(len(iov)) != n {
			db.DFatalf("NetProxyTrans mismatch between supplied destination nvec and incoming nvec: %v != %v\n%s", len(iov), n, debug.Stack())
		}
		db.DPrintf(db.NETPROXYTRANS, "[%p] Read n frames: %v", trans.conn, len(iov))
		if err := frame.ReadNFramesInto(trans.conn, iov); err != nil {
			db.DPrintf(db.NETPROXYTRANS_ERR, "Error ReadNFramesInto: %v", err)
			return nil, err
		}
	}
	db.DPrintf(db.NETPROXYTRANS, "Read n done: %v", len(iov))
	// XXX am I setting the right IOV here? In the right way?
	// Set the out blob IOV to the socket control message
	ok, err1 := trans.RecvSocketControlMsg(iov[len(iov)-1])
	if err1 != nil {
		db.DPrintf(db.NETPROXYTRANS_ERR, "Error RecvSocketControlMsg: %v", err1)
		return nil, serr.NewErrError(err1)
	}
	db.DPrintf(db.NETPROXYTRANS, "Recvd socket control msg ok %v", ok)
	// If no control message was received, set the blob's Iov to nil
	if !ok {
		iov[len(iov)-1] = nil
	}
	return NewProxyCall(seqno, iov, true), nil
}

func (trans *NetProxyTrans) WriteCall(call demux.CallI) *serr.Err {
	db.DPrintf(db.NETPROXYTRANS, " [%p] WriteCall trans %v", trans.conn, call)
	pc := call.(*ProxyCall)
	if err := frame.WriteSeqno(pc.Seqno, trans.conn); err != nil {
		db.DPrintf(db.NETPROXYTRANS_ERR, "Error WriteSeqno: %v", err)
		return err
	}
	if err := frame.WriteFrames(trans.conn, pc.Iov); err != nil {
		db.DPrintf(db.NETPROXYTRANS_ERR, "Error WriteFrames: %v", err)
		return err
	}
	var b sessp.Tframe
	if pc.sendfd {
		b = pc.Iov[len(pc.Iov)-1]
	} else {
		b = []byte{}
	}
	if err := trans.SendSocketControlMsg(b); err != nil {
		db.DPrintf(db.NETPROXYTRANS_ERR, "Error SendSocketControlMsg: %v", err)
		return serr.NewErrError(err)
	}
	return nil
}

// Receive socket control message
func (trans *NetProxyTrans) RecvSocketControlMsg(oob []byte) (bool, error) {
	db.DPrintf(db.NETPROXYTRANS, "[%p] RecvSocketControlMsg len %v", trans.conn, len(oob))
	defer db.DPrintf(db.NETPROXYTRANS, "[%p] RecvSocketControlMsg done len %v", trans.conn, len(oob))
	// Sanity check
	if len(oob) != unix.CmsgSpace(4) {
		db.DFatalf("Error oob for control message wrong size: %v != %v", len(oob), unix.CmsgSpace(4))
	}
	b := make([]byte, 1)
	// Receive socket control message
	_, oobn, _, _, err := trans.conn.ReadMsgUnix(b, oob)
	if err != nil {
		db.DPrintf(db.NETPROXYTRANS_ERR, "Error recv proxied conn fd: err %v", err)
		return false, err
	}
	if oobn == 0 {
		db.DPrintf(db.NETPROXYTRANS, "No socket control msg receive")
		return false, nil
	}
	return true, nil
}

// Send socket control message
func (trans *NetProxyTrans) SendSocketControlMsg(oob []byte) error {
	db.DPrintf(db.NETPROXYTRANS, "[%p] SendSocketControlMsg len %v", trans.conn, len(oob))
	// Send socket control message
	_, _, err := trans.conn.WriteMsgUnix([]byte{'x'}, oob, nil)
	if err != nil {
		db.DPrintf(db.NETPROXYSRV_ERR, "Error send conn fd (%v): %v", oob, err)
		return err
	}
	return nil
}

// Receive the connection FD corresponding to a successful Dial request
func (trans *NetProxyTrans) GetReturnedConn() (*net.TCPConn, error) {
	fd, ok, err := trans.getReturnedFD()
	if err != nil {
		return nil, err
	}
	// If no FD was passed, return a nil connection
	if !ok {
		return nil, nil
	}
	return fdToTCPConn(fd)
}

func (trans *NetProxyTrans) getReturnedFD() (int, bool, error) {
	oob := make([]byte, unix.CmsgSpace(4))
	// Receive connection FD via socket
	_, oobn, _, _, err := trans.conn.ReadMsgUnix(nil, oob)
	if err != nil {
		db.DPrintf(db.NETPROXYTRANS_ERR, "Error recv proxied conn fd: err %v", err)
		return 0, false, err
	}
	if oobn == 0 {
		db.DPrintf(db.NETPROXYTRANS, "No conn FD to receive")
		return 0, false, nil
	}
	scma, err := unix.ParseSocketControlMessage(oob)
	if err != nil {
		db.DFatalf("Error parse socket control message: %v", err)
	}
	fds, err := unix.ParseUnixRights(&scma[0])
	if err != nil || len(fds) != 1 {
		db.DFatalf("Error parse unix rights: len %v err %v", len(fds), err)
	}
	db.DPrintf(db.NETPROXYTRANS, "got socket fd %v", fds[0])
	return fds[0], true, nil
}
