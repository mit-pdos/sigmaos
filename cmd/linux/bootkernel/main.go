package main

import (
	"os"
	"strconv"
	"strings"

	"sigmaos/boot"
	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/kernel"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

func main() {
	if len(os.Args) < 7 {
		db.DFatalf("%v: usage kernelid srvs nameds dbip mongoip overlays\n", os.Args[0])
	}
	srvs := strings.Split(os.Args[3], ";")
	overlays, err := strconv.ParseBool(os.Args[6])
	if err != nil {
		db.DFatalf("Error parse overlays: %v", err)
	}
	param := kernel.Param{
		KernelId: os.Args[1],
		Services: srvs,
		Dbip:     os.Args[4],
		Mongoip:  os.Args[5],
		Overlays: overlays,
	}
	db.DPrintf(db.KERNEL, "param %v\n", param)
	h := sp.SIGMAHOME
	p := os.Getenv("PATH")
	os.Setenv("PATH", p+":"+h+"/bin/kernel:"+h+"/bin/linux:"+h+"/bin/user")
	//	addrs, err := sp.String2Taddrs(os.Args[2])
	//	if err != nil {
	//		db.DFatalf("%v: String2Taddrs %v\n", os.Args[0], err)
	//	}
	localIP, err1 := container.LocalIP()
	if err1 != nil {
		db.DFatalf("Error local IP: %v", err1)
	}
	pcfg := proc.NewBootProcEnv(sp.Tuname(param.KernelId), os.Args[2], localIP, param.Overlays)
	proc.SetSigmaDebugPid(pcfg.GetPID().String())
	if err := boot.BootUp(&param, pcfg); err != nil {
		db.DFatalf("%v: boot %v err %v\n", os.Args[0], os.Args[1:], err)
	}
	os.Exit(0)
}
