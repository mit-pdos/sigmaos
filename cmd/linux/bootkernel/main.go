package main

import (
	"os"

	"sigmaos/boot"
	bk "sigmaos/bootkernelclnt"
	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/kernel"
	"sigmaos/yaml"
)

func main() {
	if len(os.Args) < 2 {
		db.DFatalf("%v: usage yaml nameds\n", os.Args[0])
	}
	param := kernel.Param{}
	err := yaml.ReadYaml(os.Args[1], &param)
	if err != nil {
		db.DFatalf("%v: ReadYaml %s\n", os.Args[0], os.Args[1])
	}

	p := os.Getenv("PATH")
	os.Setenv("PATH", p+":"+bk.HOME+"/bin/kernel:"+bk.HOME+"/bin/linux:"+bk.HOME+"/bin/user")
	_, err = boot.BootUp(&param, fslib.StringToNamedAddrs(os.Args[2]))
	if err != nil {
		db.DFatalf("%v: boot %v err %v\n", os.Args[0], os.Args[1:], err)
	}
	os.Exit(0)
}
