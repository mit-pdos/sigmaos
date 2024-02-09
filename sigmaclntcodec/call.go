package sigmaclntcodec

import (
	"bufio"
	"io"

	"sigmaos/demux"
	"sigmaos/frame"
	"sigmaos/serr"
	"sigmaos/sessp"
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

func WriteCall(wrt *bufio.Writer, c demux.CallI) *serr.Err {
	fc := c.(*Call)
	if err := frame.WriteSeqno(fc.Seqno, wrt); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err.Error())
	}
	if err := frame.WriteFrames(wrt, fc.Iov); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err.Error())
	}
	return nil
}

func ReadCall(rdr io.Reader) (demux.CallI, *serr.Err) {
	seqno, err := frame.ReadSeqno(rdr)
	if err != nil {
		return nil, err
	}
	iov, err := frame.ReadFrames(rdr)
	if err != nil {
		return nil, err
	}
	return NewCall(seqno, iov), nil
}
