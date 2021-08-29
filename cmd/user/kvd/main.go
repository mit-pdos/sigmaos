package main

import (
	"log"
	"os"

	"ulambda/fslib"
	"ulambda/kv"
	"ulambda/procinit"
)

func main() {
	fsl := fslib.MakeFsLib("kvd")
	os.Setenv(procinit.SCHED_LAYERS, procinit.MakeProcLayers(map[string]bool{procinit.BASESCHED: true, procinit.DEPSCHED: true}))
	sctl := procinit.MakeProcCtl(fsl, procinit.GetProcLayers())
	conf := kv.MakeConfig(0)
	err := fsl.MakeFileJson(kv.KVCONFIG, 0777, *conf)
	if err != nil {
		log.Fatalf("Cannot make file  %v %v\n", kv.KVCONFIG, err)
	}
	pid := kv.SpawnKV(sctl)
	kv.RunBalancer(sctl, "add", pid)
}
