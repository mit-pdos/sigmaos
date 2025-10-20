package benchmarks

import (
	"time"

	cachegrpmgr "sigmaos/apps/cache/cachegrp/mgr"
	cossimsrv "sigmaos/apps/cossim/srv"
	"sigmaos/apps/hotel"
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

var DefaultCacheBenchConfig = &CacheBenchConfig{
	JobCfg: &cachegrpmgr.CacheJobConfig{
		NSrv: 1,
		MCPU: proc.Tmcpu(1000),
		GC:   true,
	},
	CPP:           false,
	RunSleeper:    false,
	CosSimBackend: false,
	UseEPCache:    false,
	DelegateInit:  false,
	Autoscale:     false,
	NKeys:         1000,
	TopNShards:    1,
	Durs:          []time.Duration{10 * time.Second},
	MaxRPS:        []int{100},
	PutDurs:       []time.Duration{5 * time.Second},
	PutMaxRPS:     []int{50},
	Scale: &ManualScalingConfig{
		Svc:        "cached",
		Scale:      false,
		ScaleDelay: 0 * time.Second,
		NToAdd:     0,
	},
}

var DefaultHotelBenchConfig = &HotelBenchConfig{
	JobCfg: &hotel.HotelJobConfig{
		Job:    "hotel-job",
		Srvs:   hotel.NewHotelSvc(),
		NHotel: 80,
		Cache:  "cached",
		CacheCfg: &cachegrpmgr.CacheJobConfig{
			NSrv: 1,
			MCPU: proc.Tmcpu(1000),
			GC:   true,
		},
		ImgSizeMB:       0,
		NGeo:            1,
		NGeoIdx:         4000,
		GeoSearchRadius: 500,
		GeoNResults:     5,
		UseMatch:        false,
	},
	MatchUseCaching: false,
	Durs:            []time.Duration{10 * time.Second},
	MaxRPS:          []int{100},
	ScaleGeo: &ManualScalingConfig{
		Svc:        "hotel-geo",
		Scale:      false,
		ScaleDelay: 0 * time.Second,
		NToAdd:     0,
	},
	CacheBenchCfg:  nil,
	CosSimBenchCfg: nil,
}
