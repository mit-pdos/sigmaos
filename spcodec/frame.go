package spcodec

import (
	"bufio"
	"bytes"

	"sigmaos/fcall"
	"sigmaos/frame"
	sp "sigmaos/sigmap"
)

// XXX cut out copy
func MarshalFcallMsg(fc fcall.Fcall, bwr *bufio.Writer) *fcall.Err {
	fcm := fc.(*sp.FcallMsg)
	var f bytes.Buffer
	if error := encode(&f, fcm); error != nil {
		return fcall.MkErr(fcall.TErrBadFcall, error.Error())
	}
	frame.WriteFrame(bwr, f.Bytes())
	return nil
}

func MarshalFcallMsgByte(fcm *sp.FcallMsg) ([]byte, *fcall.Err) {
	var f bytes.Buffer
	if error := encode(&f, fcm); error != nil {
		return nil, fcall.MkErr(fcall.TErrBadFcall, error)
	}

	return f.Bytes(), nil
}

func UnmarshalFcallMsg(frame []byte) (fcall.Fcall, *fcall.Err) {
	fm := sp.MakeFcallMsgNull()
	if err := decode(bytes.NewReader(frame), fm); err != nil {
		return nil, fcall.MkErr(fcall.TErrBadFcall, err)
	}
	return fm, nil
}
