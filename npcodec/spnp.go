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

func Np2SpQid(npqid np.Tqid9P) *sp.Tqid {
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

func Np2SpStat(npst np.Stat9P) *sp.Stat {
	spst := &sp.Stat{}
	spst.Type = uint32(npst.Type)
	spst.Dev = npst.Dev
	spst.Qid = Np2SpQid(npst.Qid)
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
