package spcodec

import (
	"bufio"
	"encoding/binary"
	"io"

	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/frame"
	"sigmaos/serr"
	"sigmaos/sessp"
)

func ReadCall(rdr io.Reader) (demux.CallI, *serr.Err) {
	f, err := frame.ReadFrame(rdr)
	if err != nil {
		return nil, err
	}
	fm := sessp.NewFcallMsgNull()
	if error := proto.Unmarshal(f, fm.Fc); error != nil {
		db.DFatalf("Decoding fcall err %v", error)
	}

	b := make(sessp.Tframe, fm.Fc.Len)
	n, error := io.ReadFull(rdr, b)
	if n != len(b) {
		return nil, serr.NewErr(serr.TErrUnreachable, error)
	}

	iov, err := frame.ReadFramesN(rdr, fm.Fc.Nvec)
	if err != nil {
		return nil, err
	}

	fm.Iov = iov

	pmm := &sessp.PartMarshaledMsg{fm, b}

	return pmm, nil
}

func WriteCall(wr io.Writer, c demux.CallI) *serr.Err {
	wrt := wr.(*bufio.Writer)
	fcm := c.(*sessp.PartMarshaledMsg)
	fcm.Fcm.Fc.Len = uint32(len(fcm.MarshaledFcm))
	fcm.Fcm.Fc.Nvec = uint32(len(fcm.Fcm.Iov))

	b, err := proto.Marshal(fcm.Fcm.Fc)
	if err != nil {
		db.DFatalf("Encoding fcall %v err %v", fcm.Fcm.Fc, err)
	}
	if err := binary.Write(wrt, binary.LittleEndian, uint32(len(b)+4)); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err)
	}
	if _, err := wrt.Write(b); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err)
	}
	if _, err := wrt.Write(fcm.MarshaledFcm); err != nil {
		return serr.NewErr(serr.TErrUnreachable, err)
	}
	for _, f := range fcm.Fcm.Iov {
		if err := frame.WriteFrame(wrt, f); err != nil {
			return err
		}
	}
	if err := wrt.Flush(); err != nil {
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
