package npcodec

import (
	"bufio"
	"io"

	db "sigmaos/debug"
	"sigmaos/fcall"
	"sigmaos/frame"
	sp "sigmaos/sigmap"
)

func MarshalFrame(fcm *sp.FcallMsg, bwr *bufio.Writer) *fcall.Err {
	sp2NpMsg(fcm)
	fc9P := to9P(fcm)
	db.DPrintf("NPCODEC", "MarshalFrame %v\n", fc9P)
	f, error := marshal1(false, fc9P)
	if error != nil {
		return fcall.MkErr(fcall.TErrBadFcall, error.Error())
	}
	if err := frame.WriteFrame(bwr, f); err != nil {
		return err
	}
	if error := bwr.Flush(); error != nil {
		db.DPrintf("NPCODEC", "flush %v err %v", fcm, error)
		return fcall.MkErr(fcall.TErrBadFcall, error.Error())
	}
	return nil
}

func UnmarshalFrame(rdr io.Reader) (*sp.FcallMsg, *fcall.Err) {
	f, err := frame.ReadFrame(rdr)
	if err != nil {
		db.DPrintf("NPCODEC", "ReadFrame err %v\n", err)
		return nil, err
	}
	fc9p := &Fcall9P{}
	if err := unmarshal(f, fc9p); err != nil {
		db.DPrintf("NPCODEC", "unmarshal err %v\n", err)
		return nil, fcall.MkErr(fcall.TErrBadFcall, err)
	}
	fc := toSP(fc9p)
	np2SpMsg(fc)
	return fc, nil
}
