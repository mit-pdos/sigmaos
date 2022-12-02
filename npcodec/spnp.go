package npcodec

import (
	np "sigmaos/ninep"
	sp "sigmaos/sigmap"
)

func Sp2NpQid(spqid sp.Tqid) np.Tqid9P {
	npqid := np.Tqid9P{}
	npqid.Type = np.Qtype9P(spqid.Type)
	npqid.Version = np.TQversion(spqid.Version)
	npqid.Path = np.Tpath(spqid.Path)
	return npqid
}

func Sp2NpStat(spst *sp.Stat) *np.Stat9P {
	npst := &np.Stat9P{}
	npst.Type = uint16(spst.Type)
	npst.Dev = spst.Dev
	npst.Qid = Sp2NpQid(*spst.Qid)
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
