package benchmarks_test

import (
	"encoding/json"
	"flag"
	"testing"

	"sigmaos/benchmarks"
	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

var CosSimBenchConfig *benchmarks.CosSimBenchConfig

var cossimBenchCfgStr string

func init() {
	flag.StringVar(&cossimBenchCfgStr, "cossim_bench_cfg", sp.NOT_SET, "JSON string for CosSimBenchConfig")
}

func TestMain(t *testing.M) {
	flag.Parse()
	if cossimBenchCfgStr != "" {
		err := json.Unmarshal([]byte(cossimBenchCfgStr), &CosSimBenchConfig)
		if err != nil {
			db.DFatalf("Error unmarshaling cossim_bench_cfg: %v", err)
		}
		db.DPrintf(db.ALWAYS, "Loaded CosSimBenchConfig: %v", CosSimBenchConfig)
	}
}
