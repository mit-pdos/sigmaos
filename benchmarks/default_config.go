package benchmarks

import (
	"path/filepath"
	"time"

	cachegrpmgr "sigmaos/apps/cache/cachegrp/mgr"
	cossimsrv "sigmaos/apps/cossim/srv"
	"sigmaos/apps/hotel"
	"sigmaos/apps/imgresize"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
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
	ManuallyScale: &ManualScalingConfig{
		Svc:         "cossim-srv",
		Scale:       false,
		ScaleDelays: []time.Duration{},
		ScaleDeltas: []int{},
	},
	Autoscale: &AutoscalingConfig{
		Svc:              "cossim-srv",
		Scale:            false,
		InitialNReplicas: 1,
		MaxReplicas:      0,
		TargetRIF:        10.0,
		Frequency:        1 * time.Second,
		Tolerance:        0.1,
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
	ManuallyScale: &ManualScalingConfig{
		Svc:         "cached",
		Scale:       false,
		ScaleDelays: []time.Duration{},
		ScaleDeltas: []int{},
	},
	Migrate: &MigrationConfig{
		Svc:              "cached",
		Migrate:          false,
		MigrationDelays:  []time.Duration{},
		MigrationTargets: []int{},
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
	CachedUserFrac:  100,
	Durs:            []time.Duration{10 * time.Second},
	MaxRPS:          []int{100},
	ScaleGeo: &ManualScalingConfig{
		Svc:         "hotel-geo",
		Scale:       false,
		ScaleDelays: []time.Duration{},
		ScaleDeltas: []int{},
	},
	CacheBenchCfg:  nil,
	CosSimBenchCfg: nil,
}

var DefaultImgBenchConfig = &ImgBenchConfig{
	JobCfg: &imgresize.ImgdJobConfig{
		Job:           "img-job",
		WorkerMcpu:    proc.Tmcpu(0),
		WorkerMem:     proc.Tmem(0),
		Persist:       false,
		NRounds:       1,
		ImgdMcpu:      proc.Tmcpu(1000),
		UseSPProxy:    false,
		UseBootScript: false,
		UseS3Clnt:     false,
	},
	InputPath:      filepath.Join(sp.S3, sp.LOCAL, "9ps3/img/1.jpg"),
	NTasks:         10,
	NInputsPerTask: 1,
	Durs:           []time.Duration{10 * time.Second},
	MaxRPS:         []int{100},
}
