package spcodec

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"

	db "sigmaos/debug"
	"sigmaos/sessp"
    "sigmaos/serr"
	"sigmaos/frame"
)

func MarshalFrame(fcm *sessp.FcallMsg, bwr *bufio.Writer) *serr.Err {
	var f bytes.Buffer
	if error := encode(&f, fcm); error != nil {
		return serr.MkErr(serr.TErrBadFcall, error.Error())
	}
	if err := frame.WriteFrame(bwr, f.Bytes()); err != nil {
		return err
	}
	if error := binary.Write(bwr, binary.LittleEndian, uint32(len(fcm.Data))); error != nil {
		return serr.MkErr(serr.TErrUnreachable, error.Error())
	}
	db.DPrintf(db.SPCODEC, "Marshal frame %v %d buf %d\n", fcm.Msg, len(f.Bytes()), len(fcm.Data))
	if len(fcm.Data) > 0 {
		if err := frame.WriteRawBuffer(bwr, fcm.Data); err != nil {
			return serr.MkErr(serr.TErrUnreachable, err.Error())
		}
	}
	if error := bwr.Flush(); error != nil {
		db.DPrintf(db.SPCODEC, "flush %v err %v", fcm, error)
	}
	return nil
}

func MarshalFrameByte(fcm *sessp.FcallMsg) ([]byte, *serr.Err) {
	var f bytes.Buffer
	wr := bufio.NewWriter(&f)

	if err := MarshalFrame(fcm, wr); err != nil {
		return nil, err
	}
	return f.Bytes(), nil
}

func UnmarshalFrame(rdr io.Reader) (*sessp.FcallMsg, *serr.Err) {
	f, err := frame.ReadFrame(rdr)
	if err != nil {
		db.DPrintf(db.SPCODEC, "ReadFrame err %v\n", err)
		return nil, err
	}
	fm := sessp.MakeFcallMsgNull()
	if err := decode(bytes.NewReader(f), fm); err != nil {
		db.DPrintf(db.SPCODEC, "Decode err %v\n", err)
		return nil, serr.MkErr(serr.TErrBadFcall, err)
	}
	db.DPrintf(db.SPCODEC, "Decode %v\n", fm)
	buf, err := frame.ReadBuf(rdr)
	if err != nil {
		db.DPrintf(db.SPCODEC, "ReadBuf err %v\n", err)
		return nil, err
	}
	fm.Data = buf
	return fm, nil
}
