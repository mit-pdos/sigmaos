package npcodec

import (
	db "sigmaos/debug"
	"sigmaos/frame"
	"sigmaos/serr"
	"sigmaos/sessp"
)

func MarshalFrame(fcm *sessp.FcallMsg) (frame.Tframe, *serr.Err) {
	sp2NpMsg(fcm)
	fc9P := to9P(fcm)
	db.DPrintf(db.NPCODEC, "MarshalFrame %v\n", fc9P)
	f, error := marshal1(false, fc9P)
	if error != nil {
		return nil, serr.NewErr(serr.TErrBadFcall, error.Error())
	}
	return f, nil
}

func UnmarshalFrame(f frame.Tframe) (sessp.Ttag, *sessp.FcallMsg, *serr.Err) {
	fc9p := &Fcall9P{}
	if err := unmarshal(f, fc9p); err != nil {
		db.DPrintf(db.NPCODEC, "unmarshal err %v\n", err)
		return 0, nil, serr.NewErr(serr.TErrBadFcall, err)
	}
	fc := toSP(fc9p)
	np2SpMsg(fc)
	return 0, fc, nil
}
