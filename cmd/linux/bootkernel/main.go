package main

import (
	"os"
	"strings"

	"sigmaos/boot"
	db "sigmaos/debug"
	"sigmaos/kernel"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

func main() {
	if len(os.Args) < 4 {
		db.DFatalf("%v: usage srvs nameds dbip\n", os.Args[0])
	}
	srvs := strings.Split(os.Args[2], ";")
	param := kernel.Param{srvs, os.Args[3]}
	db.DPrintf(db.KERNEL, "param %v\n", param)
	h := sp.SIGMAHOME
	p := os.Getenv("PATH")
	os.Setenv("PATH", p+":"+h+"/bin/kernel:"+h+"/bin/linux:"+h+"/bin/user")
	err := boot.BootUp(&param, proc.StringToNamedAddrs(os.Args[1]))
	if err != nil {
		db.DFatalf("%v: boot %v err %v\n", os.Args[0], os.Args[1:], err)
	}
	os.Exit(0)
}
