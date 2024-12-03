package transport

import (
	"bufio"
	"io"
	"net"

	//	"runtime/debug"

	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/frame"
	"sigmaos/serr"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
)

type Call struct {
	Seqno sessp.Tseqno
	Iov   sessp.IoVec
}

func NewCall(s sessp.Tseqno, iov sessp.IoVec) *Call {
	return &Call{Seqno: s, Iov: iov}
}

func (c *Call) Tag() sessp.Ttag {
	return sessp.Ttag(c.Seqno)
}

type Transport struct {
	rdr  io.Reader
	wrt  *bufio.Writer
	iovm *demux.IoVecMap
}

func NewTransport(conn net.Conn, iovm *demux.IoVecMap) *Transport {
	return &Transport{
		rdr:  bufio.NewReaderSize(conn, sp.Conf.Conn.MSG_LEN),
		wrt:  bufio.NewWriterSize(conn, sp.Conf.Conn.MSG_LEN),
		iovm: iovm,
	}
}

func (t *Transport) WriteCall(c demux.CallI) *serr.Err {
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

func (t *Transport) ReadCall() (demux.CallI, *serr.Err) {
	seqno, err := frame.ReadSeqno(t.rdr)
	if err != nil {
		return nil, err
	}
	iov, _ := t.iovm.Get(sessp.Ttag(seqno))
	if len(iov) == 0 {
		// Read frames, creating an IO vec
		iov, err = frame.ReadFrames(t.rdr)
	} else {
		var n uint32
		n, err = frame.ReadNumOfFrames(t.rdr)
		if err != nil {
			return nil, err
		}
		if uint32(len(iov)) != n {
			db.DFatalf("mismatch between supplied destination nvec and incoming nvec: %v != %v", len(iov), n)
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
