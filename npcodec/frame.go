package npcodec

import (
	"bufio"
	"encoding/binary"
	"io"

	np "ulambda/ninep"
)

// Adopted from https://github.com/docker/go-p9p/encoding.go and Go's codecs

func ReadFrame(rd io.Reader) ([]byte, *np.Err) {
	var len uint32

	if err := binary.Read(rd, binary.LittleEndian, &len); err != nil {
		return nil, np.MkErr(np.TErrUnreachable, err)
	}
	len = len - 4
	if len <= 0 {
		return nil, np.MkErr(np.TErrUnreachable, "readMsg too short")
	}
	msg := make([]byte, len)
	n, err := io.ReadFull(rd, msg)
	if n != int(len) {
		return nil, np.MkErr(np.TErrUnreachable, err)
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

func MarshalFcall(fcall np.WritableFcall, b *bufio.Writer) *np.Err {
	frame, error := marshal1(true, fcall)
	if error != nil {
		return np.MkErr(np.TErrBadFcall, error.Error())
	}
	dataBuf := false
	var data []byte
	switch fcall.GetType() {
	case np.TTwrite:
		msg := fcall.GetMsg().(np.Twrite)
		data = msg.Data
		dataBuf = true
	case np.TTwrite1:
		msg := fcall.GetMsg().(np.Twrite1)
		data = msg.Data
		dataBuf = true
	case np.TRread:
		msg := fcall.GetMsg().(np.Rread)
		data = msg.Data
		dataBuf = true
	case np.TRgetfile:
		msg := fcall.GetMsg().(np.Rgetfile)
		data = msg.Data
		dataBuf = true
	case np.TTsetfile:
		msg := fcall.GetMsg().(np.Tsetfile)
		data = msg.Data
		dataBuf = true
	case np.TTputfile:
		msg := fcall.GetMsg().(np.Tputfile)
		data = msg.Data
		dataBuf = true
	default:
	}
	if dataBuf {
		return WriteFrameAndBuf(b, frame, data)
	} else {
		return WriteFrame(b, frame)
	}
}

func MarshalFcallByte(fcall *np.Fcall) ([]byte, *np.Err) {
	if b, error := marshal(fcall); error != nil {
		return nil, np.MkErr(np.TErrBadFcall, error)
	} else {
		return b, nil
	}
}

func UnmarshalFcall(frame []byte) (*np.Fcall, *np.Err) {
	fcall := &np.Fcall{}
	if err := unmarshal(frame, fcall); err != nil {
		return nil, np.MkErr(np.TErrBadFcall, err)
	}
	return fcall, nil
}

func UnmarshalFcallWireCompat(frame []byte) (*np.Fcall, *np.Err) {
	fcallWC := &np.FcallWireCompat{}
	if err := unmarshal(frame, fcallWC); err != nil {
		return nil, np.MkErr(np.TErrBadFcall, err)
	}
	return fcallWC.ToInternal(), nil
}
