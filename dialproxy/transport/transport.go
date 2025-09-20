package transport

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"runtime/debug"
	"strconv"
	"sync"

	"golang.org/x/sys/unix"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/serr"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
	"sigmaos/util/io/demux"
	"sigmaos/util/io/frame"
)

const (
	SIGMA_DIALPROXY_FD = "SIGMA_DIALPROXY_FD"
)

var hasBeenInit bool

type DialProxyTrans struct {
	sync.Mutex
	conn      *net.UnixConn
	iovm      *demux.IoVecMap
	openConns map[sessp.Tseqno]*os.File
}

func NewDialProxyTrans(conn *net.UnixConn, iovm *demux.IoVecMap) *DialProxyTrans {
	return &DialProxyTrans{
		conn:      conn,
		iovm:      iovm,
		openConns: make(map[sessp.Tseqno]*os.File),
	}
}

func GetDialProxydConn(pe *proc.ProcEnv) (*net.UnixConn, error) {
	var conn *net.UnixConn
	fdstr := os.Getenv(SIGMA_DIALPROXY_FD)
	if fdstr == "" {
		// Connect to the dialproxy server by dialing the unix socket (should only
		// be done by the test program)
		uconn, err := net.Dial("unix", sp.SIGMA_DIALPROXY_SOCKET)
		if err != nil {
			db.DPrintf(db.ERROR, "Error connect dialproxy srv")
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
		if err := frame.WriteFrameBuf(conn, b); err != nil {
			db.DPrintf(db.ERROR, "Error WriteFrame principal: %v", err)
			return nil, err
		}
	} else {
		// Sanity check that a proc only has one DialProxyClnt, since using the fd
		// set up by the trampoline consumes it destructively.
		if hasBeenInit {
			db.DPrintf(db.ERROR, "Error double-init dialproxyclnt")
			return nil, fmt.Errorf("Error double-init dialproxyclnt: %v", string(debug.Stack()))
		}
		hasBeenInit = true
		// Connect to the dialproxy server using the FD set up by the trampoline
		// (should be done by user procs)
		fd, err := strconv.Atoi(fdstr)
		if err != nil {
			db.DPrintf(db.ERROR, "Error get dialproxy fd (%v): %v", fdstr, err)
			return nil, err
		}
		conn, err = fdToUnixConn(fd)
		if err != nil {
			db.DPrintf(db.ERROR, "Error connect dialproxy srv")
			return nil, err
		}
	}
	return conn, nil
}

func (trans *DialProxyTrans) Conn() *net.UnixConn {
	return trans.conn
}

func (trans *DialProxyTrans) Close() error {
	return trans.conn.Close()
}

func (trans *DialProxyTrans) ReadCall() (demux.CallI, error) {
	db.DPrintf(db.DIALPROXYTRANS, "ReadCall trans conn [%p]", trans.conn)
	seqno, err := frame.ReadSeqno(trans.conn)
	if err != nil {
		db.DPrintf(db.DIALPROXYTRANS_ERR, "Error ReadSeqno: %v", err)
		return nil, err
	}
	iov, _ := trans.iovm.Get(sessp.Ttag(seqno))
	if iov == nil {
		// If there are outputs, but the caller didn't supply any IoVecs to write
		// them to, create an IoVec to hold the outputs
		iov = sessp.NewUnallocatedIoVec(0, nil)
	}
	if iov.Len() == 0 {
		// Read frames, creating an IO vec
		iov, err = frame.ReadFrames(trans.conn)
		if err != nil {
			db.DPrintf(db.DIALPROXYTRANS_ERR, "Error ReadFrames: %v", err)
			return nil, err
		}
	} else {
		n, err := frame.ReadNumOfFrames(trans.conn)
		if err != nil {
			db.DPrintf(db.DIALPROXYTRANS_ERR, "Error ReadNumOfFrames: %v", err)
			return nil, err
		}
		if uint32(iov.Len()) < n {
			db.DFatalf("DialProxyTrans mismatch between supplied destination nvec and incoming nvec: %v != %v\n%s", iov.Len(), n, debug.Stack())
		}
		db.DPrintf(db.DIALPROXYTRANS, "[%p] Read n frames: %v", trans.conn, iov.Len())
		if err := frame.ReadNFramesInto(trans.conn, iov); err != nil {
			db.DPrintf(db.DIALPROXYTRANS_ERR, "Error ReadNFramesInto: %v", err)
			return nil, err
		}
	}
	db.DPrintf(db.DIALPROXYTRANS, "Read n done: %v", iov.Len())
	// Set the out blob IOV to the socket control message
	ok, err1 := trans.RecvSocketControlMsg(iov.GetFrame(iov.Len() - 1).GetBuf())
	if err1 != nil {
		db.DPrintf(db.DIALPROXYTRANS_ERR, "Error RecvSocketControlMsg: %v", err1)
		return nil, serr.NewErrError(err1)
	}
	db.DPrintf(db.DIALPROXYTRANS, "Recvd socket control msg ok %v", ok)
	// If no control message was received, set the blob's Iov to nil
	if !ok {
		iov.GetFrame(iov.Len() - 1).SetBuf(nil)
	}
	return NewProxyCall(seqno, iov), nil
}

func (trans *DialProxyTrans) WriteCall(call demux.CallI) error {
	db.DPrintf(db.DIALPROXYTRANS, "[%p] WriteCall trans %v", trans.conn, call)
	pc := call.(*ProxyCall)
	if err := frame.WriteSeqno(pc.Seqno, trans.conn); err != nil {
		db.DPrintf(db.DIALPROXYTRANS_ERR, "Error WriteSeqno: %v", err)
		return err
	}
	if err := frame.WriteFrames(trans.conn, pc.Iov); err != nil {
		db.DPrintf(db.DIALPROXYTRANS_ERR, "Error WriteFrames: %v", err)
		return err
	}
	if err := trans.SendSocketControlMsg(pc.Iov.GetFrame(pc.Iov.Len() - 1).GetBuf()); err != nil {
		db.DPrintf(db.DIALPROXYTRANS_ERR, "Error SendSocketControlMsg: %v", err)
		return serr.NewErrError(err)
	}
	if err := trans.DelConn(pc.Seqno); err != nil {
		db.DPrintf(db.DIALPROXYTRANS_ERR, "Error DelConn: %v", err)
		return serr.NewErrError(err)
	}
	return nil
}

// Receive socket control message
func (trans *DialProxyTrans) RecvSocketControlMsg(oob []byte) (bool, error) {
	db.DPrintf(db.DIALPROXYTRANS, "[%p] RecvSocketControlMsg len %v", trans.conn, len(oob))
	defer db.DPrintf(db.DIALPROXYTRANS, "[%p] RecvSocketControlMsg done len %v", trans.conn, len(oob))
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
		db.DPrintf(db.DIALPROXYTRANS_ERR, "Error recv proxied conn fd: err %v", err)
		return false, err
	}
	if oobn == 0 {
		db.DPrintf(db.DIALPROXYTRANS, "No socket control msg received")
		return false, nil
	}
	return true, nil
}

// Send socket control message
func (trans *DialProxyTrans) SendSocketControlMsg(oob []byte) error {
	db.DPrintf(db.DIALPROXYTRANS, "[%p] SendSocketControlMsg len %v", trans.conn, len(oob))
	// Send at least one byte, in case there is no socket control message to be
	// sent
	b := []byte{'x'}
	// Send socket control message
	_, _, err := trans.conn.WriteMsgUnix(b, oob, nil)
	if err != nil {
		db.DPrintf(db.DIALPROXYTRANS_ERR, "Error send conn fd (%v): %v", oob, err)
		return err
	}
	return nil
}

func (trans *DialProxyTrans) AddConn(seqno sessp.Tseqno, conn *os.File) {
	// If no connection, bail out
	if conn == nil {
		return
	}

	trans.Lock()
	defer trans.Unlock()

	// Sanity check
	if _, ok := trans.openConns[seqno]; ok {
		db.DFatalf("DialProxytrans overwrite conn: seqno %v", seqno)
	}
	trans.openConns[seqno] = conn
}

func (trans *DialProxyTrans) DelConn(seqno sessp.Tseqno) error {
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
