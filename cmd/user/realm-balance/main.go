package main

import (
	"fmt"
	"log"
	"os"
	"path"

	"ulambda/benchmarks"
	"ulambda/fslib"
	"ulambda/realm"
)

func main() {
	fsl1 := fslib.MakeFsLib("microbenchmarks-1")
	cfg := realm.GetRealmConfig(fsl1, realm.TEST_RID)
	fsl := fslib.MakeFsLibAddr("microbenchmarks", cfg.NamedAddr)

	b := benchmarks.MakeRealmBalanceBenchmark(fsl)
	b.Run()

	realmds, err := fsl1.ReadDir(path.Join(realm.REALMS, realm.TEST_RID))
	if err != nil {
		log.Fatalf("Error ReadDir realm-balance main: %v", err)
	}
	log.Printf("# realmds: %v", len(realmds))
	if len(realmds) > 2 {
		log.Printf("PASS")
	} else {
		log.Printf("FAIL")
	}
}
