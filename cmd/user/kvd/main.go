package main

import (
	"os"

	"sigmaos/apps/kv/kvgrp"
	db "sigmaos/debug"
	"sigmaos/ft/procgroupmgr"
)

func main() {
	if len(os.Args) != 3 {
		db.DFatalf("Usage: %v <jobdir> <grp>", os.Args[0])
	}
	id, nrepl, err := procgroupmgr.ParseREPL(os.Getenv("SIGMAREPL"))
	if err != nil {
		db.DFatalf("%v: err %v\n", os.Args[0], err)
	}
	kvgrp.RunMember(os.Args[1], os.Args[2], id, nrepl)
}
