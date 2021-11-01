package main

import (
	"ulambda/benchmarks"
	"ulambda/fslib"
	"ulambda/realm"
)

func main() {
	fsl1 := fslib.MakeFsLib("microbenchmarks-1")
	cfg := realm.GetRealmConfig(fsl1, realm.TEST_RID)
	fsl := fslib.MakeFsLibAddr("microbenchmarks", cfg.NamedAddr)

	b := benchmarks.MakeRealmBalanceBenchmark(fsl1, fsl)
	b.Run()
}
