package benchmarks

import (
	"time"

	cachegrpmgr "sigmaos/apps/cache/cachegrp/mgr"
	cossimsrv "sigmaos/apps/cossim/srv"
	"sigmaos/proc"
)

var DefaultCosSimBenchConfig = &CosSimBenchConfig{
	JobCfg: &cossimsrv.CosSimJobConfig{
		Job:       "cossim-job",
		InitNSrv:  1,
		NVec:      10000,
		VecDim:    128,
		EagerInit: true,
		SrvMcpu:   proc.Tmcpu(1000),
		CacheCfg: &cachegrpmgr.CacheJobConfig{
			NSrv: 1,
			MCPU: proc.Tmcpu(1000),
			GC:   true,
		},
		DelegateInitRPCs: false,
	},
	NVecToQuery: 100,
	Durs:        []time.Duration{10 * time.Second},
	MaxRPS:      []int{100},
	Scale: &ManualScalingConfig{
		Svc:        "cossim-srv",
		Scale:      false,
		ScaleDelay: 0 * time.Second,
		NToAdd:     0,
	},
}
