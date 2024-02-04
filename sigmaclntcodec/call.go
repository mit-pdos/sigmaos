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
	Data  []byte
}

func NewCall(s sessp.Tseqno, data []byte) *Call {
	return &Call{Seqno: s, Data: data}
}

func (c *Call) Tag() sessp.Ttag {
	return sessp.Ttag(c.Seqno)
}

func WriteCall(wrt *bufio.Writer, c demux.CallI) *serr.Err {
	fc := c.(*Call)
	if err := frame.WriteSeqno(fc.Seqno, wrt); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err.Error())
	}
	if err := frame.WriteFrame(wrt, fc.Data); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err.Error())
	}
	return nil
}

func ReadCall(rdr io.Reader) (demux.CallI, *serr.Err) {
	seqno, err := frame.ReadSeqno(rdr)
	if err != nil {
		return nil, err
	}
	data, err := frame.ReadFrame(rdr)
	if err != nil {
		return nil, err
	}
	return NewCall(seqno, data), nil
}
