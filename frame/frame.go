package frame

import (
	"encoding/binary"
	"io"

	"sigmaos/fcall"
)

func ReadFrame(rd io.Reader) ([]byte, *fcall.Err) {
	var len uint32

	if err := binary.Read(rd, binary.LittleEndian, &len); err != nil {
		return nil, fcall.MkErr(fcall.TErrUnreachable, err)
	}
	len = len - 4
	if len <= 0 {
		return nil, fcall.MkErr(fcall.TErrUnreachable, "readMsg too short")
	}
	msg := make([]byte, len)
	n, e := io.ReadFull(rd, msg)
	if n != int(len) {
		return nil, fcall.MkErr(fcall.TErrUnreachable, e)
	}
	return msg, nil
}

func WriteFrame(wr io.Writer, frame []byte) *fcall.Err {
	l := uint32(len(frame) + 4)

	if err := binary.Write(wr, binary.LittleEndian, l); err != nil {
		return fcall.MkErr(fcall.TErrUnreachable, err.Error())
	}
	return WriteRawBuffer(wr, frame)
}

func WriteRawBuffer(wr io.Writer, buf []byte) *fcall.Err {
	if n, err := wr.Write(buf); err != nil {
		return fcall.MkErr(fcall.TErrUnreachable, err.Error())
	} else if n < len(buf) {
		return fcall.MkErr(fcall.TErrUnreachable, "writeRawBuffer too short")
	}
	return nil
}

func WriteFrameAndBuf(wr io.Writer, frame []byte, buf []byte) *fcall.Err {
	// Adjust frame size
	l := uint32(len(frame) + 4 + len(buf) + 4)

	// Write frame
	if error := binary.Write(wr, binary.LittleEndian, l); error != nil {
		return fcall.MkErr(fcall.TErrUnreachable, error.Error())
	}
	if err := WriteRawBuffer(wr, frame); err != nil {
		return err
	}

	// Write buf
	if error := binary.Write(wr, binary.LittleEndian, uint32(len(buf))); error != nil {
		return fcall.MkErr(fcall.TErrUnreachable, error.Error())
	}
	return WriteRawBuffer(wr, buf)
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
