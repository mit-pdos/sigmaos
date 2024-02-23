package spcodec

import (
	"bufio"
	"encoding/binary"
	"io"
	"net"

	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/frame"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type Transport struct {
	rdr io.Reader
	wrt *bufio.Writer
}

func NewTransport(conn net.Conn) demux.TransportI {
	return &Transport{
		rdr: bufio.NewReaderSize(conn, sp.Conf.Conn.MSG_LEN),
		wrt: bufio.NewWriterSize(conn, sp.Conf.Conn.MSG_LEN),
	}
}

func (t *Transport) ReadCall() (demux.CallI, *serr.Err) {
	f, err := frame.ReadFrame(t.rdr)
	if err != nil {
		return nil, err
	}
	fm := sessp.NewFcallMsgNull()
	if error := proto.Unmarshal(f, fm.Fc); error != nil {
		db.DFatalf("Decoding fcall err %v", error)
	}

	b := make(sessp.Tframe, fm.Fc.Len)
	n, error := io.ReadFull(t.rdr, b)
	if n != len(b) {
		return nil, serr.NewErr(serr.TErrUnreachable, error)
	}

	iov, err := frame.ReadFramesN(t.rdr, fm.Fc.Nvec)
	if err != nil {
		return nil, err
	}

	fm.Iov = iov

	pmm := &sessp.PartMarshaledMsg{fm, b}

	return pmm, nil
}

func (t *Transport) WriteCall(c demux.CallI) *serr.Err {
	fcm := c.(*sessp.PartMarshaledMsg)
	fcm.Fcm.Fc.Len = uint32(len(fcm.MarshaledFcm))
	fcm.Fcm.Fc.Nvec = uint32(len(fcm.Fcm.Iov))

	b, err := proto.Marshal(fcm.Fcm.Fc)
	if err != nil {
		db.DFatalf("Encoding fcall %v err %v", fcm.Fcm.Fc, err)
	}
	if err := binary.Write(t.wrt, binary.LittleEndian, uint32(len(b)+4)); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err)
	}
	if _, err := t.wrt.Write(b); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err)
	}
	if _, err := t.wrt.Write(fcm.MarshaledFcm); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err)
	}
	for _, f := range fcm.Fcm.Iov {
		if err := frame.WriteFrame(t.wrt, f); err != nil {
			return err
		}
	}
	if err := t.wrt.Flush(); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err)
	}
	return nil
}

func NewPartMarshaledMsg(fcm *sessp.FcallMsg) *sessp.PartMarshaledMsg {
	b, err := proto.Marshal(fcm.Msg.(proto.Message))
	if err != nil {
		db.DFatalf("Encoding msg %v err %v", fcm.Msg, err)
	}
	return &sessp.PartMarshaledMsg{
		Fcm:          fcm,
		MarshaledFcm: b,
	}
}

func UnmarshalMsg(pfcm *sessp.PartMarshaledMsg) *serr.Err {
	msg, err := NewMsg(pfcm.Fcm.Type())
	if err != nil {
		return err
	}
	pb := msg.(proto.Message)
	if err := proto.Unmarshal(pfcm.MarshaledFcm, pb); err != nil {
		db.DFatalf("Decoding msg %v err %v", msg, err)
	}
	pfcm.Fcm.Msg = msg
	return nil
}
