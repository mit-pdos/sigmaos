package sigmaclntcodec

import (
	"bufio"
	"io"
	"net"

	// db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/frame"
	"sigmaos/serr"
	"sigmaos/sessp"
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
	rdr io.Reader
	wrt *bufio.Writer
}

func NewTransport(conn net.Conn) *Transport {
	return &Transport{
		rdr: bufio.NewReaderSize(conn, sp.Conf.Conn.MSG_LEN),
		wrt: bufio.NewWriterSize(conn, sp.Conf.Conn.MSG_LEN),
	}
}

func (t *Transport) WriteCall(c demux.CallI) *serr.Err {
	fc := c.(*Call)
	// db.DPrintf(db.TEST, "writecall %v\n", c)
	if err := frame.WriteSeqno(fc.Seqno, t.wrt); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err.Error())
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
	iov, err := frame.ReadFrames(t.rdr)
	if err != nil {
		return nil, err
	}
	c := NewCall(seqno, iov)
	// db.DPrintf(db.TEST, "readcall %v\n", c)
	return c, nil
}
