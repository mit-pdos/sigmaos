package main

import (
	"log"
	"sort"

	"ulambda/fslib"
	"ulambda/perf"
	"ulambda/realm"
)

func main() {
	fsl1 := fslib.MakeFsLib("microbenchmarks-1")
	cfg := realm.GetRealmConfig(fsl1, realm.TEST_RID)
	fsl := fslib.MakeFsLibAddr("microbenchmarks", cfg.NamedAddr)

	m := perf.MakeMicrobenchmarks(fsl)
	res := m.RunAll()
	names := []string{}
	for name, _ := range res {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		log.Printf("%v Mean: %v", name, res[name].Mean())
	}
}
