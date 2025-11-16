package benchmarks_test

import (
	"encoding/json"
	"flag"
	"os"
	"testing"

	"sigmaos/benchmarks"
	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

var CosSimBenchConfig *benchmarks.CosSimBenchConfig
var CacheBenchConfig *benchmarks.CacheBenchConfig
var HotelBenchConfig *benchmarks.HotelBenchConfig
var ImgBenchConfig *benchmarks.ImgBenchConfig

var cossimBenchCfgStr string
var cacheBenchCfgStr string
var hotelBenchCfgStr string
var imgBenchCfgStr string

func init() {
	flag.StringVar(&cossimBenchCfgStr, "cossim_bench_cfg", sp.NOT_SET, "JSON string for CosSimBenchConfig")
	flag.StringVar(&cacheBenchCfgStr, "cache_bench_cfg", sp.NOT_SET, "JSON string for CacheBenchConfig")
	flag.StringVar(&hotelBenchCfgStr, "hotel_bench_cfg", sp.NOT_SET, "JSON string for HotelBenchConfig")
	flag.StringVar(&imgBenchCfgStr, "img_bench_cfg", sp.NOT_SET, "JSON string for ImgBenchConfig")
}

func TestMain(m *testing.M) {
	flag.Parse()

	// Parse CosSimBenchConfig
	if cossimBenchCfgStr == sp.NOT_SET {
		CosSimBenchConfig = benchmarks.DefaultCosSimBenchConfig
		db.DPrintf(db.ALWAYS, "Using default CosSimBenchConfig")
	} else {
		err := json.Unmarshal([]byte(cossimBenchCfgStr), &CosSimBenchConfig)
		if err != nil {
			db.DFatalf("Error unmarshaling cossim_bench_cfg: %v", err)
		}
		db.DPrintf(db.ALWAYS, "Loaded CosSimBenchConfig")
	}

	// Parse CacheBenchConfig
	if cacheBenchCfgStr == sp.NOT_SET {
		CacheBenchConfig = benchmarks.DefaultCacheBenchConfig
		db.DPrintf(db.ALWAYS, "Using default CacheBenchConfig")
	} else {
		err := json.Unmarshal([]byte(cacheBenchCfgStr), &CacheBenchConfig)
		if err != nil {
			db.DFatalf("Error unmarshaling cache_bench_cfg: %v", err)
		}
		db.DPrintf(db.ALWAYS, "Loaded CacheBenchConfig")
	}

	// Parse HotelBenchConfig
	if hotelBenchCfgStr == sp.NOT_SET {
		HotelBenchConfig = benchmarks.DefaultHotelBenchConfig
		db.DPrintf(db.ALWAYS, "Using default HotelBenchConfig")
	} else {
		err := json.Unmarshal([]byte(hotelBenchCfgStr), &HotelBenchConfig)
		if err != nil {
			db.DFatalf("Error unmarshaling hotel_bench_cfg: %v", err)
		}
		db.DPrintf(db.ALWAYS, "Loaded HotelBenchConfig")
	}

	// Parse ImgBenchConfig
	if imgBenchCfgStr == sp.NOT_SET {
		ImgBenchConfig = benchmarks.DefaultImgBenchConfig
		db.DPrintf(db.ALWAYS, "Using default ImgBenchConfig")
	} else {
		err := json.Unmarshal([]byte(imgBenchCfgStr), &ImgBenchConfig)
		if err != nil {
			db.DFatalf("Error unmarshaling img_bench_cfg: %v", err)
		}
		db.DPrintf(db.ALWAYS, "Loaded ImgBenchConfig")
	}

	CosSimBenchConfig.JobCfg.CacheCfg = CacheBenchConfig.JobCfg
	HotelBenchConfig.JobCfg.CacheCfg = CacheBenchConfig.JobCfg
	HotelBenchConfig.CosSimBenchCfg = CosSimBenchConfig
	HotelBenchConfig.CacheBenchCfg = CacheBenchConfig

	db.DPrintf(db.ALWAYS, "CacheBenchConfig: %v", CacheBenchConfig)
	db.DPrintf(db.ALWAYS, "CosSimBenchConfig: %v", CosSimBenchConfig)
	db.DPrintf(db.ALWAYS, "HotelBenchConfig: %v", HotelBenchConfig)
	db.DPrintf(db.ALWAYS, "ImgBenchConfig: %v", ImgBenchConfig)

	os.Exit(m.Run())
}
