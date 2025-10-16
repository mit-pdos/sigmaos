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

var cossimBenchCfgStr string
var cacheBenchCfgStr string
var hotelBenchCfgStr string

func init() {
	flag.StringVar(&cossimBenchCfgStr, "cossim_bench_cfg", sp.NOT_SET, "JSON string for CosSimBenchConfig")
	flag.StringVar(&cacheBenchCfgStr, "cache_bench_cfg", sp.NOT_SET, "JSON string for CacheBenchConfig")
	flag.StringVar(&hotelBenchCfgStr, "hotel_bench_cfg", sp.NOT_SET, "JSON string for HotelBenchConfig")
}

func TestMain(m *testing.M) {
	flag.Parse()

	// Parse CosSimBenchConfig
	if cossimBenchCfgStr == sp.NOT_SET {
		CosSimBenchConfig = benchmarks.DefaultCosSimBenchConfig
		db.DPrintf(db.ALWAYS, "Using default CosSimBenchConfig: %v", CosSimBenchConfig)
	} else {
		err := json.Unmarshal([]byte(cossimBenchCfgStr), &CosSimBenchConfig)
		if err != nil {
			db.DFatalf("Error unmarshaling cossim_bench_cfg: %v", err)
		}
		db.DPrintf(db.ALWAYS, "Loaded CosSimBenchConfig: %v", CosSimBenchConfig)
	}

	// Parse CacheBenchConfig
	if cacheBenchCfgStr == sp.NOT_SET {
		CacheBenchConfig = benchmarks.DefaultCacheBenchConfig
		db.DPrintf(db.ALWAYS, "Using default CacheBenchConfig: %v", CacheBenchConfig)
	} else {
		err := json.Unmarshal([]byte(cacheBenchCfgStr), &CacheBenchConfig)
		if err != nil {
			db.DFatalf("Error unmarshaling cache_bench_cfg: %v", err)
		}
		db.DPrintf(db.ALWAYS, "Loaded CacheBenchConfig: %v", CacheBenchConfig)
	}

	// Parse HotelBenchConfig
	if hotelBenchCfgStr == sp.NOT_SET {
		HotelBenchConfig = benchmarks.DefaultHotelBenchConfig
		db.DPrintf(db.ALWAYS, "Using default HotelBenchConfig: %v", HotelBenchConfig)
	} else {
		err := json.Unmarshal([]byte(hotelBenchCfgStr), &HotelBenchConfig)
		if err != nil {
			db.DFatalf("Error unmarshaling hotel_bench_cfg: %v", err)
		}
		db.DPrintf(db.ALWAYS, "Loaded HotelBenchConfig: %v", HotelBenchConfig)
	}

	os.Exit(m.Run())
}
