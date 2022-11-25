package protdevsrv

import (
	"encoding/json"
	"fmt"
	"sync"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/fslibsrv"
	"sigmaos/inode"
	np "sigmaos/ninep"
)

type MethodStat struct {
	N   uint64 // number of invocations of method
	Tot int64  // tot us for this method
	Max int64
	Avg float64
}

func (ms *MethodStat) String() string {
	return fmt.Sprintf("N %d Tot %dus Max %dus Avg %.1fus", ms.N, ms.Tot, ms.Max, ms.Avg)
}

type Stats struct {
	MStats  map[string]*MethodStat
	AvgQLen float64
}

func mkStats() *Stats {
	st := &Stats{}
	st.MStats = make(map[string]*MethodStat)
	return st
}

func (st *Stats) String() string {
	s := "stats:\n methods:\n"
	for k, st := range st.MStats {
		s += fmt.Sprintf("  %s: %s\n", k, st.String())
	}
	s += fmt.Sprintf(" AvgQLen: %.3f", st.AvgQLen)
	return s
}

type StatInfo struct {
	sync.Mutex
	st  *Stats
	len uint64
}

func MakeStatInfo() *StatInfo {
	si := &StatInfo{}
	si.st = mkStats()
	return si
}

func (si *StatInfo) Stats() *Stats {
	n := uint64(0)
	for _, st := range si.st.MStats {
		n += st.N
		if st.N > 0 {
			st.Avg = float64(st.Tot) / float64(st.N)
		}
	}
	if n > 0 {
		si.st.AvgQLen = float64(si.len) / float64(n)
	}
	return si.st
}

func (sts *StatInfo) Stat(m string, t int64, ql int) {
	sts.Lock()
	defer sts.Unlock()
	sts.len += uint64(ql)
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

func makeStatsDev(mfs *fslibsrv.MemFs) (*StatInfo, *np.Err) {
	std := &statsDev{}
	i, err := mfs.MkDev(STATS, std)
	if err != nil {
		return nil, err
	}
	std.Inode = i
	std.si = MakeStatInfo()
	return std.si, nil
}

func (std *statsDev) Read(ctx fs.CtxI, off np.Toffset, cnt np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	if off > 0 {
		return nil, nil
	}

	std.si.Lock()
	defer std.si.Unlock()

	db.DPrintf("PROTDEVSRV", "Read stats: %v\n", std.si)
	std.si.Stats()
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
