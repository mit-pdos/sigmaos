package protdev

import (
	"fmt"
	"sync"

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
	MStats map[string]*MethodStat
}

type SigmaRPCStats struct {
	SigmaStat stats.Stats
	RpcStat   RPCStats
}

func (st *SigmaRPCStats) String() string {
	s := "Sigma stats:\n" + st.SigmaStat.String() + "\n"
	s += st.RpcStat.String()
	return s
}

func mkStats() *RPCStats {
	st := &RPCStats{}
	st.MStats = make(map[string]*MethodStat)
	return st
}

func (st *RPCStats) String() string {
	s := "RPC stats:\n methods:\n"
	for k, st := range st.MStats {
		s += fmt.Sprintf("  %s: %s\n", k, st.String())
	}
	return s
}

type StatInfo struct {
	sync.Mutex
	st *RPCStats
}

func MakeStatInfo() *StatInfo {
	si := &StatInfo{}
	si.st = mkStats()
	return si
}

func (si *StatInfo) Stats() *RPCStats {
	n := uint64(0)
	for _, st := range si.st.MStats {
		n += st.N
		if st.N > 0 {
			st.Avg = float64(st.Tot) / float64(st.N) / 1000.0
		}
	}
	return si.st
}

func (sts *StatInfo) Stat(m string, t int64) {
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
