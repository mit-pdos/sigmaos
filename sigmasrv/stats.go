package sigmasrv

import (
	"encoding/json"
	"path/filepath"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/memfs/inode"
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
	if err := mfs.MkNod(filepath.Join(pn, rpc.STATS), std); err != nil {
		return nil, err
	}
	std.si = rpc.NewStatInfo()
	return std.si, nil
}

func (std *statsDev) marshal() ([]byte, *serr.Err) {
	db.DPrintf(db.SIGMASRV, "Marshal stats: %v\n", std.si)
	st := &rpc.RPCStatsSnapshot{}
	st.StatsSnapshot = std.mfs.Stats().StatsSnapshot()
	st.RpcStat = std.si.Stats()
	b, err := json.Marshal(st)
	if err != nil {
		return nil, serr.NewErrError(err)
	}
	return b, nil
}

func (std *statsDev) Stat(ctx fs.CtxI) (*sp.Stat, *serr.Err) {
	st, err := std.Inode.NewStat()
	if err != nil {
		return nil, err
	}
	b, err := std.marshal()
	if err != nil {
		return nil, err
	}
	st.SetLengthInt(len(b))
	return st, nil
}

func (std *statsDev) Read(ctx fs.CtxI, off sp.Toffset, cnt sp.Tsize, f sp.Tfence) ([]byte, *serr.Err) {
	if off > 0 {
		return nil, nil
	}
	db.DPrintf(db.SIGMASRV, "Read stats: %v\n", std.si)
	b, err := std.marshal()
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (std *statsDev) Write(ctx fs.CtxI, off sp.Toffset, b []byte, f sp.Tfence) (sp.Tsize, *serr.Err) {
	return 0, serr.NewErr(serr.TErrNotSupported, nil)
}

func (std *statsDev) Close(ctx fs.CtxI, m sp.Tmode) *serr.Err {
	db.DPrintf(db.SIGMASRV, "Close stats\n")
	return nil
}
