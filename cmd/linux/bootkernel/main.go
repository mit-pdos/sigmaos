package main

import (
	"os"
	"strings"

	"sigmaos/boot"
	db "sigmaos/debug"
	"sigmaos/kernel"
	"sigmaos/port"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

func main() {
	if len(os.Args) < 6 {
		db.DFatalf("%v: usage kernelid srvs nameds dbip port-range\n", os.Args[0])
	}
	srvs := strings.Split(os.Args[3], ";")
	r, err := port.ParsePortRange(os.Args[5])
	if err != nil {
		db.DFatalf("%v: ParsePortRange %v err %v\n", os.Args[0], os.Args[5], err)
	}
	param := kernel.Param{os.Args[1], srvs, os.Args[4], r}
	db.DPrintf(db.KERNEL, "param %v\n", param)
	h := sp.SIGMAHOME
	p := os.Getenv("PATH")
	os.Setenv("PATH", p+":"+h+"/bin/kernel:"+h+"/bin/linux:"+h+"/bin/user")
	err = boot.BootUp(&param, proc.StringToNamedAddrs(os.Args[2]))
	if err != nil {
		db.DFatalf("%v: boot %v err %v\n", os.Args[0], os.Args[1:], err)
	}
	os.Exit(0)
}
