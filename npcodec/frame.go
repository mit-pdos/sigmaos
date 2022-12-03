package npcodec

import (
	"bufio"

	"sigmaos/fcall"
	"sigmaos/frame"
	np "sigmaos/sigmap"
)

type FcallWireCompat struct {
	Type fcall.Tfcall
	Tag  np.Ttag
	Msg  fcall.Tmsg
}

func ToInternal(fcallWC *FcallWireCompat) *np.FcallMsg {
	fm := np.MakeFcallMsgNull()
	fm.Fc.Type = uint32(fcallWC.Type)
	fm.Fc.Tag = uint32(fcallWC.Tag)
	fm.Fc.Session = uint64(fcall.NoSession)
	fm.Fc.Seqno = uint64(np.NoSeqno)
	fm.Msg = fcallWC.Msg
	return fm
}

func ToWireCompatible(fm *np.FcallMsg) *FcallWireCompat {
	fcallWC := &FcallWireCompat{}
	fcallWC.Type = fcall.Tfcall(fm.Fc.Type)
	fcallWC.Tag = np.Ttag(fm.Fc.Tag)
	fcallWC.Msg = fm.Msg
	return fcallWC
}

func MarshalFcallMsg(fcm *np.FcallMsg, b *bufio.Writer) *fcall.Err {
	f, error := marshal1(true, ToWireCompatible(fcm))
	if error != nil {
		return fcall.MkErr(fcall.TErrBadFcall, error.Error())
	}
	if fcm.Data != nil {
		return frame.WriteFrameAndBuf(b, f, fcm.Data)
	} else {
		return frame.WriteFrame(b, f)
	}
}

func UnmarshalFcallWireCompat(frame []byte) (*np.FcallMsg, *fcall.Err) {
	fcallWC := &FcallWireCompat{}
	if err := unmarshal(frame, fcallWC); err != nil {
		return nil, fcall.MkErr(fcall.TErrBadFcall, err)
	}
	return ToInternal(fcallWC), nil
}
