package benchmarks

import (
	"path/filepath"
	"time"

	cachegrpmgr "sigmaos/apps/cache/cachegrp/mgr"
	cossimsrv "sigmaos/apps/cossim/srv"
	"sigmaos/apps/etcd"
	"sigmaos/apps/hotel"
	imgrec_py "sigmaos/apps/imgrec/py"
	imgrec_wasm "sigmaos/apps/imgrec/wasm"
	"sigmaos/apps/imgresize"
	"sigmaos/apps/memcached"
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
		UseCoSandboxRPCs: false,
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
	UseCoSandbox:  false,
	Autoscale:     false,
	Shmem:         false,
	NKeys:         1000,
	ValSize:       100,
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
		Job:                  "img-job",
		WorkerMcpu:           proc.Tmcpu(0),
		WorkerMem:            proc.Tmem(0),
		Persist:              false,
		NRounds:              1,
		ImgdMcpu:             proc.Tmcpu(1000),
		UseSPProxy:           false,
		UseCoSandbox:         false,
		WriteOutViaCoSandbox: false,
		UseS3Clnt:            false,
		WorkerCoSandboxMcpu:  proc.Tmcpu(0),
		WorkerCoSandboxMem:   proc.Tmem(0),
		FTTaskSrvMcpu:        proc.Tmcpu(1000),
		ImgDim:               160,
		PremountS3:           false,
		MeasurePSS:           false,
		BailOut:              false,
	},
	InputPath:      filepath.Join(sp.S3, sp.LOCAL, "9ps3/img/8.jpg"),
	NTasks:         10,
	NInputsPerTask: 1,
	Durs:           []time.Duration{10 * time.Second},
	MaxRPS:         []int{100},
}

var DefaultEtcdBenchConfig = &EtcdBenchConfig{
	JobCfg: &etcd.EtcdJobConfig{
		Job:          "etcd-job",
		SnapshotPath: "9ps3/snapshot.db",
		Name:         "etcd-proc",
		PeerPort:     6380,
		ClientPort:   6379,
		UseCoSandbox: true,
		Mcpu:         proc.Tmcpu(1000),
	},
}

var DefaultMemcachedBenchConfig = &MemcachedBenchConfig{
	JobCfg: &memcached.MemcachedJobConfig{
		Job:          "memcached-job",
		SnapshotPath: "9ps3/memcached-snapshot-40M",
		Port:         11211,
		UseCoSandbox: false,
		Mcpu:         proc.Tmcpu(1000),
	},
	Cache: false,
}

var DefaultImgrecPyBenchConfig = &ImgrecPyBenchConfig{
	JobCfg: &imgrec_py.ImgrecPyJobConfig{
		ImgBucket:    "9ps3",
		ImgKey:       "img-save/8.jpg",
		ModelBucket:  "9ps3",
		ModelKey:     "mobilenetv2-12.onnx",
		Kid:          "~local",
		UseCoSandbox: false,
		ShmemMB:      proc.Tmem(0),
		Mcpu:         proc.Tmcpu(1000),
	},
}

var DefaultImgrecWASMBenchConfig = &ImgrecWASMBenchConfig{
	JobCfg: &imgrec_wasm.ImgrecWASMJobConfig{
		ImgBucket:         "9ps3",
		ImgKey:            "img-save/8.jpg",
		ModelBucket:       "9ps3",
		ModelKey:          "mobilenetv2-12.onnx",
		Kid:               "~local",
		UseDelegated:      false,
		UseCoSandbox:      false,
		ShmemMB:           proc.Tmem(256),
		UseWriteReadShmem: true,
		Mcpu:              proc.Tmcpu(1000),
	},
}

var DefaultStartLatencyBenchConfig = &StartLatencyBenchConfig{
	App: "etcd",
}

const (
	sebsBucket    = "9ps3"
	sebsInputBase = "serverless-benchmarks-input"
	sebsKid       = "~local"
	sebsMcpu      = proc.Tmcpu(4000)
)

var DefaultSebsThumbnailerBenchConfig = &SebsBenchConfig{
	Benchmark:    "210.thumbnailer",
	Event:        `{"bucket":{"bucket":"9ps3","input":"img-save","output":"serverless-benchmarks-input/210.thumbnailer/output/0"},"object":{"key":"test.jpg","width":200,"height":200}}`,
	Kid:          sebsKid,
	ShmemMB:      proc.Tmem(128),
	Mcpu:         sebsMcpu,
	UseCoSandbox: false,
	AsyncFetch:   true,
	Uncompressed: false,
}

var DefaultSebsVideoProcessingBenchConfig = &SebsBenchConfig{
	Benchmark:    "220.video-processing",
	Event:        `{"bucket":{"bucket":"9ps3","input":"serverless-benchmarks-input/220.video-processing/input/0","output":"serverless-benchmarks-input/220.video-processing/output/0"},"object":{"key":"city.mp4","op":"watermark","duration":1}}`,
	Kid:          sebsKid,
	ShmemMB:      proc.Tmem(128),
	Mcpu:         sebsMcpu,
	UseCoSandbox: false,
	AsyncFetch:   true,
	Uncompressed: false,
}

var DefaultSebsImageRecognitionBenchConfig = &SebsBenchConfig{
	Benchmark:    "411.image-recognition",
	Event:        `{"bucket":{"bucket":"9ps3","model":"serverless-benchmarks-input/411.image-recognition/input/0","input":"serverless-benchmarks-input/411.image-recognition/input/1"},"object":{"model":"resnet50-19c8e357.pth","input":"800px-Porsche_991_silver_IAA.jpg"}}`,
	Kid:          sebsKid,
	ShmemMB:      proc.Tmem(256),
	Mcpu:         sebsMcpu,
	UseCoSandbox: false,
	AsyncFetch:   true,
	Uncompressed: false,
}

var DefaultSebsDnaVisualisationBenchConfig = &SebsBenchConfig{
	Benchmark:    "504.dna-visualisation",
	Event:        `{"bucket":{"bucket":"9ps3","input":"serverless-benchmarks-input/504.dna-visualisation/input/0","output":"serverless-benchmarks-input/504.dna-visualisation/output/0"},"object":{"key":"test.fasta"}}`,
	Kid:          sebsKid,
	ShmemMB:      proc.Tmem(128),
	Mcpu:         sebsMcpu,
	UseCoSandbox: false,
	AsyncFetch:   true,
	Uncompressed: false,
}

var DefaultSebsBenchConfig = DefaultSebsThumbnailerBenchConfig
