package spcodec

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"

	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/frame"
	"sigmaos/serr"
	"sigmaos/sessp"
)

func MarshalFcallWithoutData(fcm *sessp.FcallMsg) []byte {
	var f bytes.Buffer
	if error := encode(&f, fcm); error != nil {
		db.DFatalf("error encoding fcall %v", error)
	}
	return f.Bytes()
}

func writeFcallAndData(fcm *sessp.FcallMsg, marshaledFcall []byte, bwr *bufio.Writer) *serr.Err {
	if err := frame.WriteFrame(bwr, marshaledFcall); err != nil {
		return err
	}
	if error := binary.Write(bwr, binary.LittleEndian, uint32(len(fcm.Data))); error != nil {
		return serr.NewErr(serr.TErrUnreachable, error.Error())
	}
	if len(fcm.Data) > 0 {
		if err := frame.WriteRawBuffer(bwr, fcm.Data); err != nil {
			return serr.NewErr(serr.TErrUnreachable, err.Error())
		}
	}
	if error := bwr.Flush(); error != nil {
		db.DPrintf(db.SPCODEC, "flush %v err %v", fcm, error)
		return serr.NewErr(serr.TErrUnreachable, error.Error())
	}
	return nil
}

func MarshalFcallAndData(fcm *sessp.FcallMsg) ([]byte, *serr.Err) {
	var f bytes.Buffer
	wr := bufio.NewWriter(&f)
	b := MarshalFcallWithoutData(fcm)
	db.DPrintf(db.SPCODEC, "Marshal frame %v %d buf %d\n", fcm.Msg, len(b), len(fcm.Data))
	if err := writeFcallAndData(fcm, b, wr); err != nil {
		return nil, err
	}
	return f.Bytes(), nil
}

func readFcallAndDataFrames(rdr io.Reader) (fc []byte, data []byte, se *serr.Err) {
	f, err := frame.ReadFrame(rdr)
	if err != nil {
		db.DPrintf(db.SPCODEC, "ReadFrame err %v\n", err)
		return nil, nil, err
	}
	buf, err := frame.ReadBuf(rdr)
	if err != nil {
		db.DPrintf(db.SPCODEC, "ReadBuf err %v\n", err)
		return nil, nil, err
	}
	return f, buf, nil
}

func UnmarshalFcallAndData(f []byte, buf []byte) *sessp.FcallMsg {
	fm := sessp.NewFcallMsgNull()
	if err := decode(bytes.NewReader(f), fm); err != nil {
		db.DFatalf("Decode error: %v", err)
	}
	db.DPrintf(db.SPCODEC, "Decode %v\n", fm)
	fm.Data = buf
	return fm
}

func readUnmarshalFcallAndData(rdr io.Reader) (*sessp.FcallMsg, *serr.Err) {
	f, buf, err := readFcallAndDataFrames(rdr)
	if err != nil {
		return nil, err
	}
	fm := UnmarshalFcallAndData(f, buf)
	return fm, nil
}

func ReadCall(rdr io.Reader) (demux.CallI, *serr.Err) {
	return readUnmarshalFcallAndData(rdr)
}

func WriteCall(wrt *bufio.Writer, c demux.CallI) *serr.Err {
	fcm := c.(*sessp.FcallMsg)
	fc := MarshalFcallWithoutData(fcm)
	return writeFcallAndData(fcm, fc, wrt)
}
