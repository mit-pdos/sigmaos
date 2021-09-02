package main

import (
	"log"

	"ulambda/fslib"
	"ulambda/kv"
	"ulambda/procinit"
)

func main() {
	fsl := fslib.MakeFsLib("kvd")
	procinit.SetProcLayers(map[string]bool{procinit.BASEPROC: true, procinit.DEPPROC: true})
	sclnt := procinit.MakeProcClnt(fsl, procinit.GetProcLayersMap())
	conf := kv.MakeConfig(0)
	err := fsl.MakeFileJson(kv.KVCONFIG, 0777, *conf)
	if err != nil {
		log.Fatalf("Cannot make file  %v %v\n", kv.KVCONFIG, err)
	}
	pid := kv.SpawnKV(sclnt)
	kv.RunBalancer(sclnt, "add", pid)
}
