package main

import (
	"os"
	"strings"

	"sigmaos/boot"
	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/kernel"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

func main() {
	if len(os.Args) < 8 {
		db.DFatalf("%v: usage kernelid srvs nameds dbip mongoip jaegerip overlays\n", os.Args[0])
	}
	srvs := strings.Split(os.Args[3], ";")
	param := kernel.Param{
		KernelId: os.Args[1],
		Services: srvs,
		Dbip:     os.Args[4],
		Mongoip:  os.Args[5],
		Jaegerip: os.Args[6],
		Overlays: os.Args[7] == "true",
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
	scfg := proc.NewBootProcEnv(sp.Tuname(param.KernelId), os.Args[2], localIP)
	proc.SetSigmaDebugPid(scfg.PID.String())
	if err := boot.BootUp(&param, scfg); err != nil {
		db.DFatalf("%v: boot %v err %v\n", os.Args[0], os.Args[1:], err)
	}
	os.Exit(0)
}
