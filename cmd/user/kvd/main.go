package main

import (
	"fmt"
	"os"
	"strconv"

	"sigmaos/cachesrv"
	db "sigmaos/debug"
	"sigmaos/group"
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
	cs := cachesrv.NewCacheSrv("")
	group.RunMember(os.Args[1], os.Args[2], public, cs)
}
