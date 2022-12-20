package frame

import (
	"encoding/binary"
	"io"

	db "sigmaos/debug"
	"sigmaos/sessp"
)

func ReadFrame(rd io.Reader) ([]byte, *sessp.Err) {
	var len uint32
	if err := binary.Read(rd, binary.LittleEndian, &len); err != nil {
		return nil, sessp.MkErr(sessp.TErrUnreachable, err)
	}
	db.DPrintf(db.FRAME, "ReadFrame %d\n", len)
	len = len - 4
	if len <= 0 {
		return nil, sessp.MkErr(sessp.TErrUnreachable, "readMsg too short")
	}
	frame := make([]byte, len)
	n, e := io.ReadFull(rd, frame)
	if n != int(len) {
		return nil, sessp.MkErr(sessp.TErrUnreachable, e)
	}
	return frame, nil
}

func ReadBuf(rd io.Reader) ([]byte, *sessp.Err) {
	var len uint32
	if err := binary.Read(rd, binary.LittleEndian, &len); err != nil {
		return nil, sessp.MkErr(sessp.TErrUnreachable, err)
	}
	var buf []byte
	if len > 0 {
		buf = make([]byte, len)
		n, e := io.ReadFull(rd, buf)
		if n != int(len) {
			return nil, sessp.MkErr(sessp.TErrUnreachable, e)
		}
	}
	return buf, nil
}

func WriteFrame(wr io.Writer, frame []byte) *sessp.Err {
	l := uint32(len(frame) + 4) // +4 because that is how 9P wants it
	if err := binary.Write(wr, binary.LittleEndian, l); err != nil {
		return sessp.MkErr(sessp.TErrUnreachable, err.Error())
	}
	return WriteRawBuffer(wr, frame)
}

func WriteRawBuffer(wr io.Writer, buf []byte) *sessp.Err {
	if n, err := wr.Write(buf); err != nil {
		return sessp.MkErr(sessp.TErrUnreachable, err.Error())
	} else if n < len(buf) {
		return sessp.MkErr(sessp.TErrUnreachable, "writeRawBuffer too short")
	}
	return nil
}

func PushToFrame(wr io.Writer, b []byte) error {
	if err := binary.Write(wr, binary.LittleEndian, uint32(len(b))); err != nil {
		return err
	}
	if err := binary.Write(wr, binary.LittleEndian, b); err != nil {
		return err
	}
	return nil
}

func PopFromFrame(rd io.Reader) ([]byte, error) {
	var l uint32
	if err := binary.Read(rd, binary.LittleEndian, &l); err != nil {
		return nil, err
	}
	b := make([]byte, int(l))
	if _, err := rd.Read(b); err != nil && !(err == io.EOF && l == 0) {
		return nil, err
	}
	return b, nil
}
