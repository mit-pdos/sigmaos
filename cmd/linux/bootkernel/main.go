package main

import (
	"os"
	"sigmaos/boot"
	db "sigmaos/debug"
	"sigmaos/kernel"
	"sigmaos/netsigma"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) < 9 {
		db.DFatalf("usage: %v kernelid srvs nameds dbip mongoip overlays provider reserveMcpu gvisor\nprovided:%v", os.Args[0], os.Args)
	}
	db.DPrintf(db.BOOT, "Boot %v", os.Args[1:])
	srvs := strings.Split(os.Args[3], ";")
	overlays, err := strconv.ParseBool(os.Args[6])
	if err != nil {
		db.DFatalf("Error parse overlays: %v", err)
	}
	gvisor, err := strconv.ParseBool(os.Args[9])
	if err != nil {
		db.DFatalf("Error parse gvisor: %v", err)
	}
	param := kernel.Param{
		KernelId: os.Args[1],
		Services: srvs,
		Dbip:     os.Args[4],
		Mongoip:  os.Args[5],
		Overlays: overlays,
		Provider: os.Args[7],
		GVisor:   gvisor,
	}
	if len(os.Args) >= 9 {
		param.ReserveMcpu = os.Args[8]
	}
	db.DPrintf(db.KERNEL, "param %v\n", param)
	h := sp.SIGMAHOME
	p := os.Getenv("PATH")
	os.Setenv("PATH", p+":"+h+"/bin/kernel:"+h+"/bin/linux:"+h+"/bin/user")
	localIP, err1 := netsigma.LocalIP()
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
