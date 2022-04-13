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
	resDir := os.Args[1]
	fpath := path.Join(resDir, "realm_balance", "results.txt")

	fslTop := fslib.MakeFsLib("realm-balance-top")
	cfg1 := realm.GetRealmConfig(fslTop, realm.TEST_RID)
	fsl1 := fslib.MakeFsLibAddr("realm-balance-1", cfg1.NamedAddrs)
	cfg2 := realm.GetRealmConfig(fslTop, "2000")
	fsl2 := fslib.MakeFsLibAddr("realm-balance-2", cfg2.NamedAddrs)

	m := benchmarks.MakeRealmBalanceBenchmark(fsl1, cfg1.NamedAddrs, fsl2, cfg2.NamedAddrs, resDir)
	res := m.Run()
	names := []string{}
	for name, _ := range res {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		log.Printf("%v Mean: %v", name, res[name].Mean())
	}
	b, err := json.Marshal(res)
	if err != nil {
		db.DFatalf("Error marshalling results: %v", err)
	}
	if err := ioutil.WriteFile(fpath, b, 0666); err != nil {
		db.DFatalf("Error WriteFile in microbenchmarks.main: %v", err)
	}
}
