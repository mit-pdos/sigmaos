package npcodec

import (
	"bufio"
	"io"

	db "sigmaos/debug"
	"sigmaos/fcall"
	"sigmaos/frame"
	np "sigmaos/ninep"
	sp "sigmaos/sigmap"
)

type Fcall9P struct {
	Type fcall.Tfcall
	Tag  sp.Ttag
	Msg  fcall.Tmsg
}

func toSP(fcallWC *Fcall9P) *sp.FcallMsg {
	fm := sp.MakeFcallMsgNull()
	fm.Fc.Type = uint32(fcallWC.Type)
	fm.Fc.Tag = uint32(fcallWC.Tag)
	fm.Fc.Session = uint64(fcall.NoSession)
	fm.Fc.Seqno = uint64(sp.NoSeqno)
	fm.Msg = fcallWC.Msg
	return fm
}

func to9P(fm *sp.FcallMsg) *Fcall9P {
	fcallWC := &Fcall9P{}
	fcallWC.Type = fcall.Tfcall(fm.Fc.Type)
	fcallWC.Tag = sp.Ttag(fm.Fc.Tag)
	fcallWC.Msg = fm.Msg
	return fcallWC
}

func MarshalFrame(fcm *sp.FcallMsg, bwr *bufio.Writer) *fcall.Err {
	if fcm.Type() == fcall.TRread {
		r := np.Rread9P{fcm.Data}
		fcm.Msg = r
		fcm.Data = nil
	}
	f, error := marshal1(false, to9P(fcm))
	if error != nil {
		return fcall.MkErr(fcall.TErrBadFcall, error.Error())
	}
	if err := frame.WriteFrame(bwr, f); err != nil {
		return err
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
	if fc9p.Type == fcall.TTread {
		m := fc.Msg.(*np.Tread)
		r := sp.MkReadV(sp.Tfid(m.Fid), sp.Toffset(m.Offset), sp.Tsize(m.Count), 0)
		fc.Msg = r
	}
	if fc9p.Type == fcall.TTwrite {
		m := fc.Msg.(*np.Twrite)
		r := sp.MkTwriteV(sp.Tfid(m.Fid), sp.Toffset(m.Offset), 0)
		fc.Msg = r
		fc.Data = m.Data
	}
	if fc9p.Type == fcall.TTopen9P {
		m := fc.Msg.(*np.Topen9P)
		r := sp.MkTopen(sp.Tfid(m.Fid), sp.Tmode(m.Mode))
		fc.Msg = r
	}
	if fc9p.Type == fcall.TTcreate9P {
		m := fc.Msg.(*np.Tcreate9P)
		r := sp.MkTcreate(sp.Tfid(m.Fid), m.Name, sp.Tperm(m.Perm), sp.Tmode(m.Mode))
		fc.Msg = r
	}
	if fc9p.Type == fcall.TTwstat9P {
		m := fc.Msg.(*np.Twstat9P)
		r := sp.MkTwstat(sp.Tfid(m.Fid), Np2SpStat(m.Stat))
		fc.Msg = r
	}
	return fc, nil
}
