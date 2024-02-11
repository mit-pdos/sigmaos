package spcodec

import (
	"bufio"
	"bytes"
	"io"

	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/frame"
	"sigmaos/serr"
	"sigmaos/sessp"
)

func MarshalFcall(fcm *sessp.FcallMsg) []byte {
	var f bytes.Buffer
	if error := encode(&f, fcm); error != nil {
		db.DFatalf("error encoding fcall %v", error)
	}
	return f.Bytes()
}

func writeFcall(fcm *sessp.FcallMsg, marshaledFcall []byte, bwr *bufio.Writer) *serr.Err {
	if err := frame.WriteFrame(bwr, marshaledFcall); err != nil {
		return err
	}
	if err := frame.WriteFrames(bwr, fcm.Iov); err != nil {
		return err
	}
	if error := bwr.Flush(); error != nil {
		db.DPrintf(db.SPCODEC, "flush %v err %v", fcm, error)
		return serr.NewErr(serr.TErrUnreachable, error.Error())
	}
	return nil
}

func readFcallAndIoVec(rdr io.Reader) (fc []byte, iov sessp.IoVec, se *serr.Err) {
	f, err := frame.ReadFrame(rdr)
	if err != nil {
		db.DPrintf(db.SPCODEC, "ReadFrame err %v\n", err)
		return nil, nil, err
	}
	iov, err = frame.ReadFrames(rdr)
	if err != nil {
		db.DPrintf(db.SPCODEC, "ReadFrames err %v\n", err)
		return nil, nil, err
	}
	return f, iov, nil
}

func UnmarshalFcall(f []byte, iov sessp.IoVec) *sessp.FcallMsg {
	fm := sessp.NewFcallMsgNull()
	if err := decode(bytes.NewReader(f), fm); err != nil {
		db.DFatalf("Decode error: %v", err)
	}
	fm.Iov = iov
	return fm
}

func readUnmarshalFcall(rdr io.Reader) (*sessp.FcallMsg, *serr.Err) {
	f, iov, err := readFcallAndIoVec(rdr)
	if err != nil {
		return nil, err
	}
	fm := UnmarshalFcall(f, iov)
	return fm, nil
}

func ReadCall(rdr io.Reader) (demux.CallI, *serr.Err) {
	return readUnmarshalFcall(rdr)
}

func WriteCall(wrt *bufio.Writer, c demux.CallI) *serr.Err {
	fcm := c.(*sessp.PartMarshaledMsg)
	return writeFcall(fcm.Fcm, fcm.MarshaledFcm, wrt)
}

func NewPartMarshaledMsg(fcm *sessp.FcallMsg) *sessp.PartMarshaledMsg {
	return &sessp.PartMarshaledMsg{
		Fcm:          fcm,
		MarshaledFcm: MarshalFcall(fcm),
	}
}
