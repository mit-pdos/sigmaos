package main

import (
	"os"
	"path"

	"sigmaos/boot"
	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/kernel"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/yaml"
)

func main() {
	if len(os.Args) < 3 {
		db.DFatalf("%v: usage yml nameds\n", os.Args[0])
	}
	param := kernel.Param{}
	pn := path.Join(container.HOSTTMP, os.Args[1])
	db.DPrintf(db.KERNEL, "kernel boot %s\n", pn)
	err := yaml.ReadYaml(pn, &param)
	if err != nil {
		db.DFatalf("%v: ReadYaml %s err %v\n", os.Args[0], pn, err)
	}
	db.DPrintf(db.KERNEL, "param %v\n", param)
	param.Realm = sp.ROOTREALM
	h := container.HOME
	p := os.Getenv("PATH")
	os.Setenv("PATH", p+":"+h+"/bin/kernel:"+h+"/bin/linux:"+h+"/bin/user")
	err = boot.BootUp(&param, proc.StringToNamedAddrs(os.Args[2]))
	if err != nil {
		db.DFatalf("%v: boot %v err %v\n", os.Args[0], os.Args[1:], err)
	}
	os.Exit(0)
}
