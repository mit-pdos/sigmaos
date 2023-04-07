package main

import (
	"os"
	"strconv"
	dbg "sigmaos/debug"
	sn "sigmaos/socialnetwork"
)

func main() {
	if len(os.Args) != 2 {
		dbg.DPrintf(dbg.SOCIAL_NETWORK_USER, "Usage: %v public", os.Args[0])
		return
	}
	public, err := strconv.ParseBool(os.Args[1])
	if err != nil {
		dbg.DFatalf("ParseBool %v err %v\n", os.Args[0], err)
	}
	if err := sn.RunUserSrv(public); err != nil {
		dbg.DFatalf("RunUserSrv %v err %v\n", os.Args[0], err)
	}
}
