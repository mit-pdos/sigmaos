package npcodec

import (
	"sigmaos/fcall"
	np "sigmaos/ninep"
	sp "sigmaos/sigmap"
)

type Fcall9P struct {
	Type fcall.Tfcall
	Tag  sp.Ttag
	Msg  fcall.Tmsg
}

func sp2NpQid(spqid sp.Tqid) np.Tqid9P {
	npqid := np.Tqid9P{}
	npqid.Type = np.Qtype9P(spqid.Type)
	npqid.Version = np.TQversion(spqid.Version)
	npqid.Path = np.Tpath(spqid.Path)
	return npqid
}

func np2SpQid(npqid np.Tqid9P) *sp.Tqid {
	spqid := &sp.Tqid{}
	spqid.Type = uint32(npqid.Type)
	spqid.Version = uint32(npqid.Version)
	spqid.Path = uint64(npqid.Path)
	return spqid
}

func Sp2NpStat(spst *sp.Stat) *np.Stat9P {
	npst := &np.Stat9P{}
	npst.Type = uint16(spst.Type)
	npst.Dev = spst.Dev
	npst.Qid = sp2NpQid(*spst.Qid)
	npst.Mode = np.Tperm(spst.Mode)
	npst.Atime = spst.Atime
	npst.Mtime = spst.Mtime
	npst.Length = np.Tlength(spst.Length)
	npst.Name = spst.Name
	npst.Uid = spst.Uid
	npst.Gid = spst.Gid
	npst.Muid = spst.Muid
	return npst
}

func Np2SpStat(npst np.Stat9P) *sp.Stat {
	spst := &sp.Stat{}
	spst.Type = uint32(npst.Type)
	spst.Dev = npst.Dev
	spst.Qid = np2SpQid(npst.Qid)
	spst.Mode = uint32(npst.Mode)
	spst.Atime = npst.Atime
	spst.Mtime = npst.Mtime
	spst.Length = uint64(npst.Length)
	spst.Name = npst.Name
	spst.Uid = npst.Uid
	spst.Gid = npst.Gid
	spst.Muid = npst.Muid
	return spst
}

func to9P(fm *sp.FcallMsg) *Fcall9P {
	fcall9P := &Fcall9P{}
	fcall9P.Type = fcall.Tfcall(fm.Fc.Type)
	fcall9P.Tag = sp.Ttag(fm.Fc.Tag)
	fcall9P.Msg = fm.Msg
	return fcall9P
}

func toSP(fcall9P *Fcall9P) *sp.FcallMsg {
	fm := sp.MakeFcallMsgNull()
	fm.Fc.Type = uint32(fcall9P.Type)
	fm.Fc.Tag = uint32(fcall9P.Tag)
	fm.Fc.Session = uint64(fcall.NoSession)
	fm.Fc.Seqno = uint64(sp.NoSeqno)
	fm.Msg = fcall9P.Msg
	return fm
}

func np2SpMsg(fcm *sp.FcallMsg) {
	switch fcm.Type() {
	case fcall.TTread:
		m := fcm.Msg.(*np.Tread)
		r := sp.MkReadV(sp.Tfid(m.Fid), sp.Toffset(m.Offset), sp.Tsize(m.Count), 0)
		fcm.Msg = r
	case fcall.TTwrite:
		m := fcm.Msg.(*np.Twrite)
		r := sp.MkTwriteV(sp.Tfid(m.Fid), sp.Toffset(m.Offset), 0)
		fcm.Msg = r
		fcm.Data = m.Data
	case fcall.TTopen9P:
		m := fcm.Msg.(*np.Topen9P)
		r := sp.MkTopen(sp.Tfid(m.Fid), sp.Tmode(m.Mode))
		fcm.Msg = r
	case fcall.TTcreate9P:
		m := fcm.Msg.(*np.Tcreate9P)
		r := sp.MkTcreate(sp.Tfid(m.Fid), m.Name, sp.Tperm(m.Perm), sp.Tmode(m.Mode))
		fcm.Msg = r
	case fcall.TTwstat9P:
		m := fcm.Msg.(*np.Twstat9P)
		r := sp.MkTwstat(sp.Tfid(m.Fid), Np2SpStat(m.Stat))
		fcm.Msg = r
	}
}

func sp2NpMsg(fcm *sp.FcallMsg) {
	switch fcm.Type() {
	case fcall.TRread:
		fcm.Fc.Type = uint32(fcall.TRread9P)
		fcm.Msg = np.Rread9P{Data: fcm.Data}
		fcm.Data = nil
	case fcall.TRerror:
		fcm.Fc.Type = uint32(fcall.TRerror9P)
		m := fcm.Msg.(*sp.Rerror)
		fcm.Msg = np.Rerror9P{Ename: fcall.Terror(m.ErrCode).String()}
	}

}
