package npcodec

import (
	"bufio"

	"sigmaos/fcall"
	"sigmaos/frame"
	np "sigmaos/sigmap"
)

func MarshalFcallMsg(fc fcall.Fcall, b *bufio.Writer) *fcall.Err {
	fcm := fc.(*np.FcallMsg)
	f, error := marshal1(true, fcm.ToWireCompatible())
	if error != nil {
		return fcall.MkErr(fcall.TErrBadFcall, error.Error())
	}
	dataBuf := false
	var data []byte
	switch fcm.Type() {
	case fcall.TTwrite:
		msg := fcm.Msg.(*np.Twrite)
		data = msg.Data
		dataBuf = true
	case fcall.TTwriteV:
		msg := fcm.Msg.(*np.TwriteV)
		data = msg.Data
		dataBuf = true
	case fcall.TRread:
		msg := fcm.Msg.(*np.Rread)
		data = msg.Data
		dataBuf = true
	case fcall.TRgetfile:
		msg := fcm.Msg.(*np.Rgetfile)
		data = msg.Data
		dataBuf = true
	case fcall.TTsetfile:
		msg := fcm.Msg.(*np.Tsetfile)
		data = msg.Data
		dataBuf = true
	case fcall.TTputfile:
		msg := fcm.Msg.(*np.Tputfile)
		data = msg.Data
		dataBuf = true
	case fcall.TTwriteread:
		msg := fcm.Msg.(*np.Twriteread)
		data = msg.Data
		dataBuf = true
	case fcall.TRwriteread:
		msg := fcm.Msg.(*np.Rwriteread)
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

func UnmarshalFcallWireCompat(frame []byte) (fcall.Fcall, *fcall.Err) {
	fcallWC := &np.FcallWireCompat{}
	if err := unmarshal(frame, fcallWC); err != nil {
		return nil, fcall.MkErr(fcall.TErrBadFcall, err)
	}
	return fcallWC.ToInternal(), nil
}
