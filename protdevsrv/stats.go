package protdevsrv

import (
	"encoding/json"
	"sync"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/inode"
	np "sigmaos/ninep"
)

type MethodStat struct {
	N   uint64 // number of invocations of method
	Tot int64  // tot us for this method
}

type Stats struct {
	sync.Mutex
	stats map[string]*MethodStat
}

func MkStats() *Stats {
	s := &Stats{}
	s.stats = make(map[string]*MethodStat)
	return s
}

func (sts *Stats) stat(m string, t int64) {
	sts.Lock()
	defer sts.Unlock()
	st, ok := sts.stats[m]
	if !ok {
		st = &MethodStat{}
		sts.stats[m] = st
	}
	st.N += 1
	st.Tot += t
}

type statsDev struct {
	*inode.Inode
	sts *Stats
}

func makeStatsDev(ctx fs.CtxI, root fs.Dir, st *Stats) fs.Inode {
	i := inode.MakeInode(ctx, np.DMDEVICE, root)
	return &statsDev{i, st}
}

func (std *statsDev) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	db.DPrintf("PROTDEVSRV", "Read stats: %v\n", std.sts)
	if off > 0 {
		return nil, nil
	}
	b, err := json.Marshal(std.sts.stats)
	if err != nil {
		return nil, np.MkErrError(err)
	}
	return b, nil
}

func (std *statsDev) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	return 0, np.MkErr(np.TErrNotSupported, nil)
}

func (std *statsDev) Close(ctx fs.CtxI, m np.Tmode) *np.Err {
	db.DPrintf("PROTDEVSRV", "Close stats\n")
	return nil
}
