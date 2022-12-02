package spcodec

import (
	"bufio"

	"sigmaos/fcall"
	"sigmaos/frame"
	sp "sigmaos/sigmap"
)

func MarshalFcallMsg(fc fcall.Fcall, b *bufio.Writer) *fcall.Err {
	fcm := fc.(*sp.FcallMsg)
	f, error := marshal1(true, fcm)
	if error != nil {
		return fcall.MkErr(fcall.TErrBadFcall, error.Error())
	}
	dataBuf := false
	var data []byte
	switch fcm.Type() {
	case fcall.TTwrite:
		msg := fcm.Msg.(*sp.Twrite)
		data = msg.Data
		dataBuf = true
	default:
	}
	if dataBuf {
		return frame.WriteFrameAndBuf(b, f, data)
	} else {
		return frame.WriteFrame(b, f)
	}
}

func MarshalFcallMsgByte(fcm *sp.FcallMsg) ([]byte, *fcall.Err) {
	if b, error := marshal(fcm); error != nil {
		return nil, fcall.MkErr(fcall.TErrBadFcall, error)
	} else {
		return b, nil
	}
}

func UnmarshalFcallMsg(frame []byte) (fcall.Fcall, *fcall.Err) {
	fm := sp.MakeFcallMsgNull()

	if err := unmarshal(frame, fm); err != nil {
		return nil, fcall.MkErr(fcall.TErrBadFcall, err)
	}
	return fm, nil
}
