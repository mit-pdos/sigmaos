package spcodec

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"

	db "sigmaos/debug"
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

func WriteSeqno(seqno sessp.Tseqno, bwr *bufio.Writer) *serr.Err {
	sn := uint64(seqno)
	if err := binary.Write(bwr, binary.LittleEndian, sn); err != nil {
		return serr.MkErr(serr.TErrUnreachable, err.Error())
	}
	return nil
}

func ReadSeqno(rdr io.Reader) (sessp.Tseqno, *serr.Err) {
	var sn uint64
	if err := binary.Read(rdr, binary.LittleEndian, &sn); err != nil {
		return 0, serr.MkErr(serr.TErrUnreachable, err.Error())
	}
	return sessp.Tseqno(sn), nil
}

func WriteFcallAndData(fcm *sessp.FcallMsg, marshaledFcall []byte, bwr *bufio.Writer) *serr.Err {
	if err := WriteSeqno(fcm.Seqno(), bwr); err != nil {
		return err
	}
	if err := frame.WriteFrame(bwr, marshaledFcall); err != nil {
		return err
	}
	if error := binary.Write(bwr, binary.LittleEndian, uint32(len(fcm.Data))); error != nil {
		return serr.MkErr(serr.TErrUnreachable, error.Error())
	}
	if len(fcm.Data) > 0 {
		if err := frame.WriteRawBuffer(bwr, fcm.Data); err != nil {
			return serr.MkErr(serr.TErrUnreachable, err.Error())
		}
	}
	if error := bwr.Flush(); error != nil {
		db.DPrintf(db.SPCODEC, "flush %v err %v", fcm, error)
		return serr.MkErr(serr.TErrUnreachable, error.Error())
	}
	return nil
}

func MarshalFcallAndData(fcm *sessp.FcallMsg) ([]byte, *serr.Err) {
	var f bytes.Buffer
	wr := bufio.NewWriter(&f)
	b := MarshalFcallWithoutData(fcm)
	db.DPrintf(db.SPCODEC, "Marshal frame %v %d buf %d\n", fcm.Msg, len(b), len(fcm.Data))
	if err := WriteFcallAndData(fcm, b, wr); err != nil {
		return nil, err
	}
	return f.Bytes(), nil
}

func ReadFcallAndDataFrames(rdr io.Reader) (sn sessp.Tseqno, fc []byte, data []byte, se *serr.Err) {
	seqno, err := ReadSeqno(rdr)
	if err != nil {
		db.DPrintf(db.SPCODEC, "ReadSeqno err %v\n", err)
		return 0, nil, nil, err
	}
	f, err := frame.ReadFrame(rdr)
	if err != nil {
		db.DPrintf(db.SPCODEC, "ReadFrame err %v\n", err)
		return 0, nil, nil, err
	}
	buf, err := frame.ReadBuf(rdr)
	if err != nil {
		db.DPrintf(db.SPCODEC, "ReadBuf err %v\n", err)
		return 0, nil, nil, err
	}
	return seqno, f, buf, nil
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

func ReadUnmarshalFcallAndData(rdr io.Reader) (sessp.Tseqno, *sessp.FcallMsg, *serr.Err) {
	seqno, f, buf, err := ReadFcallAndDataFrames(rdr)
	if err != nil {
		return 0, nil, err
	}
	fm := UnmarshalFcallAndData(f, buf)
	return seqno, fm, nil
}
