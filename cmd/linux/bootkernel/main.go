package main

import (
	"os"
	"strconv"
	"strings"

	"sigmaos/auth"
	"sigmaos/boot"
	db "sigmaos/debug"
	"sigmaos/fsetcd"
	"sigmaos/kernel"
	"sigmaos/netsigma"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

func main() {
	if len(os.Args) != 11 {
		db.DFatalf("usage: %v kernelid srvs nameds dbip mongoip overlays reserveMcpu buildTag gvisor netproxy provided:%v", os.Args[0], os.Args)
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
	netproxy, err := strconv.ParseBool(os.Args[10])
	if err != nil {
		db.DFatalf("Error parse netproxy: %v", err)
	}
	param := kernel.Param{
		KernelID: os.Args[1],
		Services: srvs,
		Dbip:     os.Args[4],
		Mongoip:  os.Args[5],
		Overlays: overlays,
		NetProxy: netproxy,
		BuildTag: os.Args[8],
		GVisor:   gvisor,
	}
	if len(os.Args) >= 8 {
		param.ReserveMcpu = os.Args[7]
	}
	db.DPrintf(db.KERNEL, "param %v", param)
	h := sp.SIGMAHOME
	p := os.Getenv("PATH")
	os.Setenv("PATH", p+":"+h+"/bin/kernel:"+h+"/bin/linux:"+h+"/bin/user")
	localIP, err1 := netsigma.LocalIP()
	if err1 != nil {
		db.DFatalf("Error local IP: %v", err1)
	}
	s3secrets, err := auth.GetAWSSecrets(sp.AWS_PROFILE)
	if err != nil {
		db.DFatalf("Failed to load AWS secrets %v", err)
	}
	etcdMnt, err := fsetcd.NewFsEtcdEndpoint(sp.Tip(os.Args[2]))
	if err != nil {
		db.DFatalf("Error NewFsEtcdEndpoint: %v", err)
	}
	secrets := map[string]*sp.SecretProto{"s3": s3secrets}
	pe := proc.NewBootProcEnv(sp.NewPrincipal(sp.TprincipalID(param.KernelID), sp.ROOTREALM), secrets, etcdMnt, localIP, localIP, param.BuildTag, param.Overlays)
	proc.SetSigmaDebugPid(pe.GetPID().String())
	if err := boot.BootUp(&param, pe); err != nil {
		db.DFatalf("%v: boot %v err %v", os.Args[0], os.Args[1:], err)
	}
	os.Exit(0)
}
