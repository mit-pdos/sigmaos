package main

import (
	"fmt"
	"os"
	"strconv"

	"sigmaos/apps/kv/kvgrp"
	db "sigmaos/debug"
	"sigmaos/groupmgr"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: %v <jobdir> <grp> <public>\n", os.Args[0])
		os.Exit(1)
	}
	public, err := strconv.ParseBool(os.Args[3])
	if err != nil {
		db.DFatalf("%v: err %v\n", os.Args[0], err)
	}
	id, nrepl, err := groupmgr.ParseREPL(os.Getenv("SIGMAREPL"))
	if err != nil {
		db.DFatalf("%v: err %v\n", os.Args[0], err)
	}
	kvgrp.RunMember(os.Args[1], os.Args[2], public, id, nrepl)
}
