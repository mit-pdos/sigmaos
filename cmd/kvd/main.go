package main

import (
	"log"

	"ulambda/fslib"
	"ulambda/kv"
	"ulambda/proc"
)

func main() {
	fsl := fslib.MakeFsLib("kvd")
	pctl := proc.MakeProcCtl(fsl, "kvd")
	conf := kv.MakeConfig(0)
	err := fsl.MakeFileJson(kv.KVCONFIG, 0777, *conf)
	if err != nil {
		log.Fatalf("Cannot make file  %v %v\n", kv.KVCONFIG, err)
	}
	pid := kv.SpawnKV(pctl)
	kv.RunBalancer(pctl, "add", pid)
}
