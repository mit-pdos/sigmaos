package main

import (
	"fmt"
	"os"
	"strconv"

	"sigmaos/cachesrv"
	db "sigmaos/debug"
	"sigmaos/kvgrp"
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
	nrepl := 0
	nrepl, err = strconv.Atoi(os.Getenv("SIGMAREPL"))
	if err != nil {
		db.DFatalf("invalid sigmarepl: %v", err)
	}
	cs := cachesrv.NewCacheSrv("", nrepl)
	kvgrp.RunMember(os.Args[1], os.Args[2], public, nrepl, cs)
}
