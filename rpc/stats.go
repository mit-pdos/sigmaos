package rpc

import (
	"fmt"
	"sync"

	"sigmaos/stats"
)

type MethodStat struct {
	N   stats.Tcounter // number of invocations of method
	Tot stats.Tcounter // tot us for this method
	Max stats.Tcounter
	Avg float64
}

// For reading and marshaling
type MethodStatSnapshot struct {
	N   int64
	Tot int64
	Max int64
	Avg float64
}

func (ms *MethodStatSnapshot) String() string {
	return fmt.Sprintf("N %d Tot %dus Max %dus Avg %.1fms", ms.N, ms.Tot, ms.Max, ms.Avg)
}

type RPCStats struct {
	MStats sync.Map
}

type RPCStatsSnapshot struct {
	*stats.StatsSnapshot
	RpcStat map[string]*MethodStatSnapshot
}

func (st *RPCStatsSnapshot) String() string {
	s := "Sigma stats:\n" + st.StatsSnapshot.String() + "\n"
	s += "RPC stats:\n methods:\n"
	for m, st := range st.RpcStat {
		s += fmt.Sprintf("  %s: %s\n", m, st.String())
	}
	return s
}

func newStats() *RPCStats {
	return &RPCStats{}
}

type StatInfo struct {
	st *RPCStats
}

func NewStatInfo() *StatInfo {
	si := &StatInfo{
		st: newStats(),
	}
	return si
}

func (si *StatInfo) Stats() map[string]*MethodStatSnapshot {
	sto := make(map[string]*MethodStatSnapshot)
	n := int64(0)
	si.st.MStats.Range(func(key, value any) bool {
		k := key.(string)
		st := value.(*MethodStat)
		stN := st.N.Load()
		stTot := st.Tot.Load()
		stMax := st.Max.Load()
		n += stN
		var avg float64
		if stN > 0 {
			avg = float64(stTot) / float64(stN) / 1000.0
		}
		ms := &MethodStatSnapshot{
			Avg: avg,
			N:   stN,
			Tot: stTot,
			Max: stMax,
		}
		sto[k] = ms
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
	stats.Inc(&st.N, 1)
	stats.Inc(&st.Tot, t)
	stats.Max(&st.Max, t)
}
