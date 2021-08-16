package main

import (
	"log"

	"ulambda/fslib"
	"ulambda/jobsched"
	"ulambda/kv"
)

func main() {
	fsl := fslib.MakeFsLib("kvd")
	sctl := jobsched.MakeSchedCtl(fsl, jobsched.DEFAULT_JOB_ID)
	conf := kv.MakeConfig(0)
	err := fsl.MakeFileJson(kv.KVCONFIG, 0777, *conf)
	if err != nil {
		log.Fatalf("Cannot make file  %v %v\n", kv.KVCONFIG, err)
	}
	pid := kv.SpawnKV(sctl)
	kv.RunBalancer(sctl, "add", pid)
}
