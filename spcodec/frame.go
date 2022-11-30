package spcodec

import (
	"bufio"
	"encoding/binary"
	"io"

	"sigmaos/fcall"
	sp "sigmaos/sigmap"
)

// Adopted from https://github.com/docker/go-p9p/encoding.go and Go's codecs

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
	n, err := io.ReadFull(rd, msg)
	if n != int(len) {
		return nil, fcall.MkErr(fcall.TErrUnreachable, err)
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

func MarshalFcallMsg(fc sp.WritableFcall, b *bufio.Writer) *fcall.Err {
	frame, error := marshal1(true, fc)
	if error != nil {
		return fcall.MkErr(fcall.TErrBadFcall, error.Error())
	}
	dataBuf := false
	var data []byte
	switch fc.GetType() {
	case fcall.TTwrite:
		msg := fc.GetMsg().(*sp.Twrite)
		data = msg.Data
		dataBuf = true
	case fcall.TTwriteV:
		msg := fc.GetMsg().(*sp.TwriteV)
		data = msg.Data
		dataBuf = true
	case fcall.TRread:
		msg := fc.GetMsg().(*sp.Rread)
		data = msg.Data
		dataBuf = true
	case fcall.TRgetfile:
		msg := fc.GetMsg().(*sp.Rgetfile)
		data = msg.Data
		dataBuf = true
	case fcall.TTsetfile:
		msg := fc.GetMsg().(*sp.Tsetfile)
		data = msg.Data
		dataBuf = true
	case fcall.TTputfile:
		msg := fc.GetMsg().(*sp.Tputfile)
		data = msg.Data
		dataBuf = true
	case fcall.TTwriteread:
		msg := fc.GetMsg().(*sp.Twriteread)
		data = msg.Data
		dataBuf = true
	case fcall.TRwriteread:
		msg := fc.GetMsg().(*sp.Rwriteread)
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

func MarshalFcallMsgByte(fcm *sp.FcallMsg) ([]byte, *fcall.Err) {
	if b, error := marshal(fcm); error != nil {
		return nil, fcall.MkErr(fcall.TErrBadFcall, error)
	} else {
		return b, nil
	}
}

func UnmarshalFcallMsg(frame []byte) (*sp.FcallMsg, *fcall.Err) {
	fm := sp.MakeFcallMsgNull()

	if err := unmarshal(frame, fm); err != nil {
		return nil, fcall.MkErr(fcall.TErrBadFcall, err)
	}
	return fm, nil
}

func UnmarshalFcallWireCompat(frame []byte) (*sp.FcallMsg, *fcall.Err) {
	fcallWC := &sp.FcallWireCompat{}
	if err := unmarshal(frame, fcallWC); err != nil {
		return nil, fcall.MkErr(fcall.TErrBadFcall, err)
	}
	return fcallWC.ToInternal(), nil
}
