package sigmasrv

import (
	"encoding/json"
	"path"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/inode"
	"sigmaos/memfssrv"
	"sigmaos/rpc"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type statsDev struct {
	mfs *memfssrv.MemFs
	*inode.Inode
	si *rpc.StatInfo
}

// Create a StatsDev in mfs at pn
func newStatsDev(mfs *memfssrv.MemFs, pn string) (*rpc.StatInfo, *serr.Err) {
	std := &statsDev{mfs: mfs, Inode: mfs.NewDevInode()}
	if err := mfs.MkDev(path.Join(pn, rpc.STATS), std); err != nil {
		return nil, err
	}
	std.si = rpc.NewStatInfo()
	return std.si, nil
}

func (std *statsDev) Read(ctx fs.CtxI, off sp.Toffset, cnt sp.Tsize, f sp.Tfence) ([]byte, *serr.Err) {
	if off > 0 {
		return nil, nil
	}

	db.DPrintf(db.SIGMASRV, "Read stats: %v\n", std.si)
	st := &rpc.SigmaRPCStats{}
	st.SigmapStat = std.mfs.GetStats().StatsCopy()
	st.RpcStat = std.si.Stats()
	b, err := json.Marshal(st)
	if err != nil {
		return nil, serr.MkErrError(err)
	}
	return b, nil
}

func (std *statsDev) Write(ctx fs.CtxI, off sp.Toffset, b []byte, f sp.Tfence) (sp.Tsize, *serr.Err) {
	return 0, serr.MkErr(serr.TErrNotSupported, nil)
}

func (std *statsDev) Close(ctx fs.CtxI, m sp.Tmode) *serr.Err {
	db.DPrintf(db.SIGMASRV, "Close stats\n")
	return nil
}
