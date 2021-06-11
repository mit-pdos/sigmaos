package main

import (
	"log"

	"ulambda/fslib"
	"ulambda/kv"
)

func main() {
	fsl := fslib.MakeFsLib("kvd")
	conf := kv.MakeConfig(0)
	err := fsl.MakeFileJson(kv.KVCONFIG, 0777, *conf)
	if err != nil {
		log.Fatalf("Cannot make file  %v %v\n", kv.KVCONFIG, err)
	}
	pid := kv.SpawnKV(fsl)
	kv.RunBalancer(fsl, "add", pid)

}
