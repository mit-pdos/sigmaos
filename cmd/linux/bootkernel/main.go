package main

import (
	"os"
	"strings"

	"sigmaos/boot"
	db "sigmaos/debug"
	"sigmaos/kernel"
	sp "sigmaos/sigmap"
)

func main() {
	if len(os.Args) < 5 {
		db.DFatalf("%v: usage kernelid srvs nameds dbip\n", os.Args[0])
	}
	srvs := strings.Split(os.Args[3], ";")
	param := kernel.Param{os.Args[1], srvs, os.Args[4]}
	db.DPrintf(db.KERNEL, "param %v\n", param)
	h := sp.SIGMAHOME
	p := os.Getenv("PATH")
	os.Setenv("PATH", p+":"+h+"/bin/kernel:"+h+"/bin/linux:"+h+"/bin/user")
	if err := boot.BootUp(&param, sp.String2Taddrs(os.Args[2])); err != nil {
		db.DFatalf("%v: boot %v err %v\n", os.Args[0], os.Args[1:], err)
	}
	os.Exit(0)
}
