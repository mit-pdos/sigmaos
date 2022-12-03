package spcodec

import (
	"bufio"
	"bytes"

	"sigmaos/fcall"
	"sigmaos/frame"
	sp "sigmaos/sigmap"
)

func MarshalFcallMsg(fcm *sp.FcallMsg, bwr *bufio.Writer) *fcall.Err {
	var f bytes.Buffer
	if error := encode(&f, fcm); error != nil {
		return fcall.MkErr(fcall.TErrBadFcall, error.Error())
	}
	return frame.WriteFrameAndBuf(bwr, f.Bytes(), fcm.Data)
}

func MarshalFcallMsgByte(fcm *sp.FcallMsg) ([]byte, *fcall.Err) {
	var f bytes.Buffer
	if error := encode(&f, fcm); error != nil {
		return nil, fcall.MkErr(fcall.TErrBadFcall, error)
	}

	return f.Bytes(), nil
}

func UnmarshalFcallMsg(frame []byte) (*sp.FcallMsg, *fcall.Err) {
	fm := sp.MakeFcallMsgNull()
	if err := decode(bytes.NewReader(frame), fm); err != nil {
		return nil, fcall.MkErr(fcall.TErrBadFcall, err)
	}
	return fm, nil
}
