package npcodec

import (
	"bufio"
	"io"

	db "sigmaos/debug"
	"sigmaos/fcall"
	"sigmaos/frame"
	np9p "sigmaos/ninep"
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

func MarshalFrame(fcm *np.FcallMsg, bwr *bufio.Writer) *fcall.Err {
	f, error := marshal1(false, ToWireCompatible(fcm))
	if error != nil {
		return fcall.MkErr(fcall.TErrBadFcall, error.Error())
	}
	if err := frame.WriteFrame(bwr, f); err != nil {
		return err
	}
	return nil
}

func UnmarshalFrame(rdr io.Reader) (*np.FcallMsg, *fcall.Err) {
	f, err := frame.ReadFrame(rdr)
	if err != nil {
		db.DPrintf("NPCODEC", "ReadFrame err %v\n", err)
		return nil, err
	}
	fc9p := &FcallWireCompat{}
	if err := unmarshal(f, fc9p); err != nil {
		db.DPrintf("NPCODEC", "unmarshal err %v\n", err)
		return nil, fcall.MkErr(fcall.TErrBadFcall, err)
	}
	fc := ToInternal(fc9p)
	if fc9p.Type == fcall.TTread {
		m := fc.Msg.(*np9p.Tread)
		r := np.MkReadV(np.Tfid(m.Fid), np.Toffset(m.Offset), np.Tsize(m.Count), 0)
		fc.Msg = r
	}
	if fc9p.Type == fcall.TTwrite {
		m := fc.Msg.(*np9p.Twrite)
		r := np.MkTwriteV(np.Tfid(m.Fid), np.Toffset(m.Offset), 0)
		fc.Msg = r
		fc.Data = m.Data
	}
	if fc9p.Type == fcall.TTopen9P {
		m := fc.Msg.(*np9p.Topen9P)
		r := np.MkTopen(np.Tfid(m.Fid), np.Tmode(m.Mode))
		fc.Msg = r
	}
	if fc9p.Type == fcall.TTcreate9P {
		m := fc.Msg.(*np9p.Tcreate9P)
		r := np.MkTcreate(np.Tfid(m.Fid), m.Name, np.Tperm(m.Perm), np.Tmode(m.Mode))
		fc.Msg = r
	}
	if fc9p.Type == fcall.TTwstat9P {
		m := fc.Msg.(*np9p.Twstat9P)
		r := np.MkTwstat(np.Tfid(m.Fid), Np2SpStat(m.Stat))
		fc.Msg = r
	}
	return fc, nil
}
