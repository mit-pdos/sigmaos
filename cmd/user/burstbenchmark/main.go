package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path"
	"sort"

	"ulambda/benchmarks"
	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/realm"
)

func main() {
	if len(os.Args) < 2 {
		db.DFatalf("Usage: %v result_dir", os.Args[0])
	}
	nReplicas := os.Getenv("N_REPLICAS")
	resDir := os.Args[1]
	fpath := path.Join(resDir, "burstbenchmark", nReplicas+"_replicas.txt")

	fsl1 := fslib.MakeFsLib("burstbenchmark-1")
	cfg := realm.GetRealmConfig(fsl1, realm.TEST_RID)
	fsl := fslib.MakeFsLibAddr("burstbenchmark", cfg.NamedAddrs)

	b := benchmarks.MakeBurstBenchmark(fsl, cfg.NamedAddrs, resDir)
	res := b.Run()
	names := []string{}
	for name, _ := range res {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		log.Printf("%v Mean: %v", name, res[name].Mean())
	}
	b1, err := json.Marshal(res)
	if err != nil {
		db.DFatalf("Error marshalling results: %v", err)
	}

	if err := ioutil.WriteFile(fpath, b1, 0666); err != nil {
		db.DFatalf("Error WriteFile in microbenchmarks.main: %v", err)
	}
}
