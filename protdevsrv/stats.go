package protdevsrv

import (
	"encoding/json"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/inode"
	"sigmaos/memfssrv"
	"sigmaos/protdev"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type statsDev struct {
	mfs *memfssrv.MemFs
	*inode.Inode
	si *protdev.StatInfo
}

func makeStatsDev(mfs *memfssrv.MemFs) (*protdev.StatInfo, *serr.Err) {
	std := &statsDev{mfs: mfs, Inode: mfs.MakeDevInode()}
	if err := mfs.MkDev(protdev.STATS, std); err != nil {
		return nil, err
	}
	std.si = protdev.MakeStatInfo()
	return std.si, nil
}

func (std *statsDev) Read(ctx fs.CtxI, off sp.Toffset, cnt sessp.Tsize, v sp.TQversion) ([]byte, *serr.Err) {
	if off > 0 {
		return nil, nil
	}

	db.DPrintf(db.PROTDEVSRV, "Read stats: %v\n", std.si)
	st := &protdev.SigmaRPCStats{}
	st.SigmapStat = std.mfs.GetStats().StatsCopy()
	st.RpcStat = std.si.Stats()
	b, err := json.Marshal(st)
	if err != nil {
		return nil, serr.MkErrError(err)
	}
	return b, nil
}

func (std *statsDev) Write(ctx fs.CtxI, off sp.Toffset, b []byte, v sp.TQversion) (sessp.Tsize, *serr.Err) {
	return 0, serr.MkErr(serr.TErrNotSupported, nil)
}

func (std *statsDev) Close(ctx fs.CtxI, m sp.Tmode) *serr.Err {
	db.DPrintf(db.PROTDEVSRV, "Close stats\n")
	return nil
}
