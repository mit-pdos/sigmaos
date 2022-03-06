package npcodec

import (
	"encoding/binary"
	"io"

	np "ulambda/ninep"
)

// Adopted from https://github.com/docker/go-p9p/encoding.go and Go's codecs

func ReadFrame(rd io.Reader) ([]byte, *np.Err) {
	var len uint32

	if err := binary.Read(rd, binary.LittleEndian, &len); err != nil {
		return nil, np.MkErr(np.TErrUnreachable, err.Error())
	}
	len = len - 4
	if len <= 0 {
		return nil, np.MkErr(np.TErrUnreachable, "readMsg too short")
	}
	msg := make([]byte, len)
	n, err := io.ReadFull(rd, msg)
	if n != int(len) {
		return nil, np.MkErr(np.TErrUnreachable, err.Error())
	}
	return msg, nil
}

func WriteFrame(wr io.Writer, frame []byte) *np.Err {
	l := uint32(len(frame) + 4)

	if err := binary.Write(wr, binary.LittleEndian, l); err != nil {
		return np.MkErr(np.TErrUnreachable, err.Error())
	}
	return WriteRawBuffer(wr, frame)
}

func WriteRawBuffer(wr io.Writer, buf []byte) *np.Err {
	if n, err := wr.Write(buf); err != nil {
		return np.MkErr(np.TErrUnreachable, err.Error())
	} else if n < len(buf) {
		return np.MkErr(np.TErrUnreachable, "writeRawBuffer too short")
	}
	return nil
}

func WriteFrameAndBuf(wr io.Writer, frame []byte, buf []byte) *np.Err {
	// Adjust frame size
	l := uint32(len(frame) + 4 + len(buf) + 4)

	// Write frame
	if error := binary.Write(wr, binary.LittleEndian, l); error != nil {
		return np.MkErr(np.TErrUnreachable, error.Error())
	}
	if err := WriteRawBuffer(wr, frame); err != nil {
		return err
	}

	// Write buf
	if error := binary.Write(wr, binary.LittleEndian, uint32(len(buf))); error != nil {
		return np.MkErr(np.TErrUnreachable, error.Error())
	}
	return WriteRawBuffer(wr, buf)
}
