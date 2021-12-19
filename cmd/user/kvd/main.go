package main

import (
	"log"

	"ulambda/fslib"
	"ulambda/kv"
	"ulambda/named"
	"ulambda/procclnt"
	"ulambda/realm"
)

func main() {
	fsl1 := fslib.MakeFsLib("kvd-1")
	cfg := realm.GetRealmConfig(fsl1, realm.TEST_RID)

	fsl := fslib.MakeFsLibAddr("kvd", cfg.NamedAddr)
	sclnt := procclnt.MakeProcClntInit(fsl, cfg.NamedAddr)

	// Set up some dirs
	fsl.Mkdir(kv.KVDIR, 0777)
	fsl.Mkdir(named.MEMFS, 0777)

	conf := kv.MakeConfig(0)
	err := fsl.MakeFileJson(kv.KVCONFIG, 0777, *conf)
	if err != nil {
		log.Fatalf("Cannot make file  %v %v\n", kv.KVCONFIG, err)
	}
	pid := kv.SpawnKV(sclnt)
	kv.BalancerOp(fsl, "add", pid)
}
