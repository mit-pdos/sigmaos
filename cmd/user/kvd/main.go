package main

import (
	"log"

	"ulambda/depproc"
	"ulambda/fslib"
	"ulambda/kv"
)

func main() {
	fsl := fslib.MakeFsLib("kvd")
	sctl := depproc.MakeDepProcCtl(fsl, depproc.DEFAULT_JOB_ID)
	conf := kv.MakeConfig(0)
	err := fsl.MakeFileJson(kv.KVCONFIG, 0777, *conf)
	if err != nil {
		log.Fatalf("Cannot make file  %v %v\n", kv.KVCONFIG, err)
	}
	pid := kv.SpawnKV(sctl)
	kv.RunBalancer(sctl, "add", pid)
}
