package protdev

import (
	"fmt"
	"sync"
	"sync/atomic"

	"sigmaos/stats"
)

type MethodStat struct {
	N   uint64 // number of invocations of method
	Tot int64  // tot us for this method
	Max int64
	Avg float64
}

func (ms *MethodStat) String() string {
	return fmt.Sprintf("N %d Tot %dus Max %dus Avg %.1fms", ms.N, ms.Tot, ms.Max, ms.Avg)
}

type RPCStats struct {
	MStats sync.Map
}

type SigmaRPCStats struct {
	SigmapStat stats.Stats
	RpcStat    RPCStats
}

func (st *SigmaRPCStats) String() string {
	s := "Sigma stats:\n" + st.SigmapStat.String() + "\n"
	s += st.RpcStat.String()
	return s
}

func mkStats() *RPCStats {
	return &RPCStats{}
}

func (st *RPCStats) String() string {
	s := "RPC stats:\n methods:\n"
	st.MStats.Range(func(key, value any) bool {
		k := key.(string)
		st := value.(*MethodStat)
		s += fmt.Sprintf("  %s: %s\n", k, st.String())
		return true
	})
	return s
}

type StatInfo struct {
	st *RPCStats
}

func MakeStatInfo() *StatInfo {
	si := &StatInfo{}
	si.st = mkStats()
	return si
}

func (si *StatInfo) Stats() *RPCStats {
	n := uint64(0)
	si.st.MStats.Range(func(key, value any) bool {
		st := value.(*MethodStat)
		stN := atomic.LoadUint64(&st.N)
		stTot := atomic.LoadInt64(&st.Tot)
		n += stN
		if st.N > 0 {
			st.Avg = float64(stTot) / float64(stN) / 1000.0
		}
		return true
	})
	return si.st
}

func (sts *StatInfo) Stat(m string, t int64) {
	var st *MethodStat
	stif, ok := sts.st.MStats.Load(m)
	if !ok {
		st = &MethodStat{}
		stif, _ = sts.st.MStats.LoadOrStore(m, st)
	}
	st = stif.(*MethodStat)
	atomic.AddUint64(&st.N, 1)
	atomic.AddInt64(&st.Tot, t)
	oldMax := atomic.LoadInt64(&st.Max)
	if oldMax == 0 || t > oldMax {
		atomic.StoreInt64(&st.Max, t)
	}
}
