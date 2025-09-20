package npcodec

import (
	"bufio"
	"io"
	"net"

	db "sigmaos/debug"
	"sigmaos/serr"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
	"sigmaos/util/io/demux"
	"sigmaos/util/io/frame"
)

type Transport struct {
	rdr  io.Reader
	wrt  *bufio.Writer
	conn net.Conn
}

func NewTransport(conn net.Conn) demux.TransportI {
	return &Transport{
		rdr:  bufio.NewReaderSize(conn, sp.Conf.Conn.MSG_LEN),
		wrt:  bufio.NewWriterSize(conn, sp.Conf.Conn.MSG_LEN),
		conn: conn,
	}
}

func (t *Transport) Close() error {
	return t.conn.Close()
}

func marshalFrame(fcm *sessp.FcallMsg) ([]byte, *serr.Err) {
	sp2NpMsg(fcm)
	fc9P := to9P(fcm)
	db.DPrintf(db.NPCODEC, "MarshalFrame %v\n", fc9P)
	f, error := marshal1(false, fc9P)
	if error != nil {
		return nil, serr.NewErr(serr.TErrBadFcall, error.Error())
	}
	return f, nil
}

func unmarshalFrame(f *sessp.Tframe) (*sessp.FcallMsg, *serr.Err) {
	fc9p := &Fcall9P{}
	if err := unmarshal(f.GetBuf(), fc9p); err != nil {
		db.DPrintf(db.NPCODEC, "unmarshal err %v\n", err)
		return nil, serr.NewErr(serr.TErrBadFcall, err)
	}
	fc := toSP(fc9p)
	np2SpMsg(fc)
	return fc, nil
}

func (t *Transport) ReadCall() (demux.CallI, error) {
	f, err := frame.ReadFrame(t.rdr)
	if err != nil {
		db.DPrintf(db.NPCODEC, "ReadFrame err %v\n", err)
		return nil, err
	}
	c, sr := unmarshalFrame(f)
	if err != nil {
		return nil, sr
	}
	return c, nil
}

func (t *Transport) WriteCall(c demux.CallI) error {
	fcm := c.(*sessp.FcallMsg)
	b, err := marshalFrame(fcm)
	if err != nil {
		return err
	}
	if err := frame.WriteFrameBuf(t.wrt, b); err != nil {
		return err
	}
	if err := t.wrt.Flush(); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err.Error())
	}
	return nil
}
