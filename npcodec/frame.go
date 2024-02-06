package npcodec

import (
	"bufio"
	"io"

	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/frame"
	"sigmaos/serr"
	"sigmaos/sessp"
)

func marshalFrame(fcm *sessp.FcallMsg) (frame.Tframe, *serr.Err) {
	sp2NpMsg(fcm)
	fc9P := to9P(fcm)
	db.DPrintf(db.NPCODEC, "MarshalFrame %v\n", fc9P)
	f, error := marshal1(false, fc9P)
	if error != nil {
		return nil, serr.NewErr(serr.TErrBadFcall, error.Error())
	}
	return f, nil
}

func unmarshalFrame(f frame.Tframe) (*sessp.FcallMsg, *serr.Err) {
	fc9p := &Fcall9P{}
	if err := unmarshal(f, fc9p); err != nil {
		db.DPrintf(db.NPCODEC, "unmarshal err %v\n", err)
		return nil, serr.NewErr(serr.TErrBadFcall, err)
	}
	fc := toSP(fc9p)
	np2SpMsg(fc)
	return fc, nil
}

func ReadCall(rdr io.Reader) (demux.CallI, *serr.Err) {
	f, err := frame.ReadFrame(rdr)
	if err != nil {
		db.DPrintf(db.NPCODEC, "ReadFrame err %v\n", err)
		return nil, err
	}
	return unmarshalFrame(f)
}

func WriteCall(wrt *bufio.Writer, c demux.CallI) *serr.Err {
	fcm := c.(*sessp.FcallMsg)
	b, err := marshalFrame(fcm)
	if err != nil {
		return err
	}
	if err := frame.WriteFrame(wrt, b); err != nil {
		return err
	}
	return nil
}
