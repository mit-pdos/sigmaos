package rpc

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
	RpcStat    map[string]*MethodStat
}

func (st *SigmaRPCStats) String() string {
	s := "Sigma stats:\n" + st.SigmapStat.String() + "\n"
	s += "RPC stats:\n methods:\n"
	for m, st := range st.RpcStat {
		s += fmt.Sprintf("  %s: %s\n", m, st.String())
	}
	return s
}

func newStats() *RPCStats {
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

func NewStatInfo() *StatInfo {
	si := &StatInfo{}
	si.st = newStats()
	return si
}

func (si *StatInfo) Stats() map[string]*MethodStat {
	sto := make(map[string]*MethodStat)
	n := uint64(0)
	si.st.MStats.Range(func(key, value any) bool {
		k := key.(string)
		st := value.(*MethodStat)
		stN := atomic.LoadUint64(&st.N)
		stTot := atomic.LoadInt64(&st.Tot)
		stMax := atomic.LoadInt64(&st.Max)
		n += stN
		var avg float64
		if st.N > 0 {
			avg = float64(stTot) / float64(stN) / 1000.0
		}
		sto[k] = &MethodStat{
			N:   stN,
			Tot: stTot,
			Max: stMax,
			Avg: avg,
		}
		return true
	})
	return sto
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
