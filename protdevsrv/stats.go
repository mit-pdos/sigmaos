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
	Max int64
	Avg float64
}

type Stats struct {
	MStats map[string]*MethodStat
	Len    uint64
}

type StatInfo struct {
	sync.Mutex
	st *Stats
}

func MkStats() *StatInfo {
	si := &StatInfo{}
	si.st = &Stats{}
	si.st.MStats = make(map[string]*MethodStat)
	return si
}

func (sts *StatInfo) queuelen(ql int) {
	sts.Lock()
	defer sts.Unlock()
	sts.st.Len += uint64(ql)
}

func (sts *StatInfo) stat(m string, t int64) {
	sts.Lock()
	defer sts.Unlock()
	st, ok := sts.st.MStats[m]
	if !ok {
		st = &MethodStat{}
		sts.st.MStats[m] = st
	}
	st.N += 1
	st.Tot += t
	if st.Max == 0 || t > st.Max {
		st.Max = t
	}
}

type statsDev struct {
	*inode.Inode
	si *StatInfo
}

func makeStatsDev(ctx fs.CtxI, root fs.Dir, si *StatInfo) fs.Inode {
	i := inode.MakeInode(ctx, np.DMDEVICE, root)
	return &statsDev{i, si}
}

func (std *statsDev) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	db.DPrintf("PROTDEVSRV", "Read stats: %v\n", std.si)
	if off > 0 {
		return nil, nil
	}
	for _, st := range std.si.st.MStats {
		if st.N > 0 {
			st.Avg = float64(st.Tot) / float64(st.N)
		}
	}
	b, err := json.Marshal(std.si.st)
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
