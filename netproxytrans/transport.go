package netproxytrans

import (
	"encoding/json"
	//"fmt"
	"net"
	"os"
	"runtime/debug"
	"strconv"
	"sync"

	"golang.org/x/sys/unix"

	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/frame"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

const (
	SIGMA_NETPROXY_FD = "SIGMA_NETPROXY_FD"
)

//var hasBeenInit bool

type NetProxyTrans struct {
	sync.Mutex
	conn      *net.UnixConn
	iovm      *demux.IoVecMap
	openConns map[sessp.Tseqno]*os.File
}

func NewNetProxyTrans(conn *net.UnixConn, iovm *demux.IoVecMap) *NetProxyTrans {
	return &NetProxyTrans{
		conn:      conn,
		iovm:      iovm,
		openConns: make(map[sessp.Tseqno]*os.File),
	}
}

func GetNetproxydConn(pe *proc.ProcEnv) (*net.UnixConn, error) {
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
		b, err := json.Marshal(pe.GetPrincipal())
		if err != nil {
			db.DFatalf("Error marshal principal: %v", err)
			return nil, err
		}
		// Write the principal ID to the server, so that the server
		// knows the principal associated with this connection. For non-test
		// programs, this will be done by the trampoline.
		if err := frame.WriteFrame(conn, b); err != nil {
			db.DPrintf(db.ERROR, "Error WriteFrame principal: %v", err)
			return nil, err
		}
	} else {
		// Sanity check that a proc only has one NetProxyClnt, since using the fd
		// set up by the trampoline consumes it destructively.
		//if hasBeenInit {
		// db.DPrintf(db.ERROR, "Error double-init netproxyclnt")
		// return nil, fmt.Errorf("Error double-init netproxyclnt: %v", string(debug.Stack()))
		//}
		//hasBeenInit = true
		// Connect to the netproxy server using the FD set up by the trampoline
		// (should be done by user procs)
		db.DPrintf(db.ALWAYS, "fdstr %v\n", fdstr)
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
	return conn, nil
}

func (trans *NetProxyTrans) Conn() *net.UnixConn {
	return trans.conn
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
		if uint32(len(iov)) < n {
			db.DFatalf("NetProxyTrans mismatch between supplied destination nvec and incoming nvec: %v != %v\n%s", len(iov), n, debug.Stack())
		}
		db.DPrintf(db.NETPROXYTRANS, "[%p] Read n frames: %v", trans.conn, len(iov))
		if err := frame.ReadNFramesInto(trans.conn, iov); err != nil {
			db.DPrintf(db.NETPROXYTRANS_ERR, "Error ReadNFramesInto: %v", err)
			return nil, err
		}
	}
	db.DPrintf(db.NETPROXYTRANS, "Read n done: %v", len(iov))
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
	return NewProxyCall(seqno, iov), nil
}

func (trans *NetProxyTrans) WriteCall(call demux.CallI) *serr.Err {
	db.DPrintf(db.NETPROXYTRANS, "[%p] WriteCall trans %v", trans.conn, call)
	pc := call.(*ProxyCall)
	if err := frame.WriteSeqno(pc.Seqno, trans.conn); err != nil {
		db.DPrintf(db.NETPROXYTRANS_ERR, "Error WriteSeqno: %v", err)
		return err
	}
	if err := frame.WriteFrames(trans.conn, pc.Iov); err != nil {
		db.DPrintf(db.NETPROXYTRANS_ERR, "Error WriteFrames: %v", err)
		return err
	}
	if err := trans.SendSocketControlMsg(pc.Iov[len(pc.Iov)-1]); err != nil {
		db.DPrintf(db.NETPROXYTRANS_ERR, "Error SendSocketControlMsg: %v", err)
		return serr.NewErrError(err)
	}
	if err := trans.DelConn(pc.Seqno); err != nil {
		db.DPrintf(db.NETPROXYTRANS_ERR, "Error DelConn: %v", err)
		return serr.NewErrError(err)
	}
	return nil
}

// Receive socket control message
func (trans *NetProxyTrans) RecvSocketControlMsg(oob []byte) (bool, error) {
	db.DPrintf(db.NETPROXYTRANS, "[%p] RecvSocketControlMsg len %v", trans.conn, len(oob))
	defer db.DPrintf(db.NETPROXYTRANS, "[%p] RecvSocketControlMsg done len %v", trans.conn, len(oob))
	// Sanity check
	if len(oob) > 0 && len(oob) != unix.CmsgSpace(4) {
		db.DPrintf(db.ERROR, "Error oob for control message wrong size: %v != %v", len(oob), unix.CmsgSpace(4))
		db.DFatalf("Error oob for control message wrong size: %v != %v", len(oob), unix.CmsgSpace(4))
	}
	// Receive at least 1 byte
	b := make([]byte, 1)
	// Receive socket control message
	_, oobn, _, _, err := trans.conn.ReadMsgUnix(b, oob)
	if err != nil {
		db.DPrintf(db.NETPROXYTRANS_ERR, "Error recv proxied conn fd: err %v", err)
		return false, err
	}
	if oobn == 0 {
		db.DPrintf(db.NETPROXYTRANS, "No socket control msg received")
		return false, nil
	}
	return true, nil
}

// Send socket control message
func (trans *NetProxyTrans) SendSocketControlMsg(oob []byte) error {
	db.DPrintf(db.NETPROXYTRANS, "[%p] SendSocketControlMsg len %v", trans.conn, len(oob))
	// Send at least one byte, in case there is no socket control message to be
	// sent
	b := []byte{'x'}
	// Send socket control message
	_, _, err := trans.conn.WriteMsgUnix(b, oob, nil)
	if err != nil {
		db.DPrintf(db.NETPROXYTRANS_ERR, "Error send conn fd (%v): %v", oob, err)
		return err
	}
	return nil
}

func (trans *NetProxyTrans) AddConn(seqno sessp.Tseqno, conn *os.File) {
	// If no connection, bail out
	if conn == nil {
		return
	}

	trans.Lock()
	defer trans.Unlock()

	// Sanity check
	if _, ok := trans.openConns[seqno]; ok {
		db.DFatalf("Netproxytrans overwrite conn: seqno %v", seqno)
	}
	trans.openConns[seqno] = conn
}

func (trans *NetProxyTrans) DelConn(seqno sessp.Tseqno) error {
	trans.Lock()
	defer trans.Unlock()

	// If no conn associated with this seqno, bail out
	conn, ok := trans.openConns[seqno]
	if !ok {
		return nil
	}
	delete(trans.openConns, seqno)
	return conn.Close()
}
