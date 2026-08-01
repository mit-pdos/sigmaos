package transport

import (
	"bufio"
	"io"
	"net"

	//	"runtime/debug"

	db "sigmaos/debug"
	"sigmaos/serr"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
	"sigmaos/util/io/demux"
	"sigmaos/util/io/frame"
)

type Call struct {
	Seqno sessp.Tseqno
	Iov   *sessp.IoVec
}

func NewCall(s sessp.Tseqno, iov *sessp.IoVec) *Call {
	return &Call{Seqno: s, Iov: iov}
}

func (c *Call) Tag() sessp.Ttag {
	return sessp.Ttag(c.Seqno)
}

type Transport struct {
	rdr  io.Reader
	wrt  *bufio.Writer
	iovm *demux.IoVecMap
	conn net.Conn
}

func NewTransport(conn net.Conn, iovm *demux.IoVecMap) *Transport {
	return &Transport{
		rdr:  bufio.NewReaderSize(conn, sp.Conf.Conn.MSG_LEN),
		wrt:  bufio.NewWriterSize(conn, sp.Conf.Conn.MSG_LEN),
		iovm: iovm,
		conn: conn,
	}
}

func (t *Transport) Close() error {
	return t.conn.Close()

}

func (t *Transport) WriteCall(c demux.CallI) error {
	fc := c.(*Call)
	// db.DPrintf(db.TEST, "writecall %v\n", c)
	if err := frame.WriteSeqno(fc.Seqno, t.wrt); err != nil {
		return err
	}
	if err := frame.WriteFrames(t.wrt, fc.Iov); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err.Error())
	}
	if err := t.wrt.Flush(); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err.Error())
	}
	return nil
}

func (t *Transport) ReadCall() (demux.CallI, error) {
	seqno, err := frame.ReadSeqno(t.rdr)
	if err != nil {
		return nil, err
	}
	iov, _ := t.iovm.Get(sessp.Ttag(seqno))
	if iov == nil {
		// If there are outputs, but the caller didn't supply any IoVecs to write
		// them to, create an IoVec to hold the outputs
		iov = sessp.NewUnallocatedIoVec(0, nil)
	}
	if iov.Len() == 0 {
		// Read frames, creating an IO vec
		iov, err = frame.ReadFrames(t.rdr)
	} else {
		var n uint32
		n, err = frame.ReadNumOfFrames(t.rdr)
		if err != nil {
			return nil, err
		}
		// Sanity check: the caller must have supplied at least as many
		// destinations as the reply has frames, or there is nowhere to put them.
		if uint32(iov.Len()) < n {
			db.DFatalf("mismatch between supplied destination nvec and incoming nvec: %v < %v", iov.Len(), n)
		}
		// Fewer frames than destinations is normal and must not be fatal: an
		// error reply carries only the error, without the result message or its
		// blob. Trim to what actually arrived so the error reaches the caller
		// instead of being reported here as a frame-count mismatch. (The
		// session/codec transport does the same.)
		if uint32(iov.Len()) > n {
			iov.TruncateFrames(int(n))
		}
		// Read frames into the IoVec
		err = frame.ReadNFramesInto(t.rdr, iov)
	}
	if err != nil {
		return nil, err
	}

	c := NewCall(seqno, iov)
	// db.DPrintf(db.TEST, "readcall %v\n", c)
	return c, nil
}
