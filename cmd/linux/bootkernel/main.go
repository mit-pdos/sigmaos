package main

import (
	"os"
	"strconv"
	"strings"

	ini "sigmaos/boot/init"
	db "sigmaos/debug"
	"sigmaos/kernel"
	"sigmaos/namesrv/fsetcd"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/util/auth"
	iputil "sigmaos/util/ip"
)

func main() {
	if len(os.Args) != 11 {
		db.DFatalf("usage: %v kernelid srvs nameds dbip mongoip reserveMcpu buildTag dialproxy net user\nprovided:%v", os.Args[0], os.Args)
	}
	db.DPrintf(db.BOOT, "Boot %v", os.Args[1:])
	srvs := strings.Split(os.Args[3], ";")
	dialproxy, err := strconv.ParseBool(os.Args[8])
	if err != nil {
		db.DFatalf("Error parse dialproxy: %v", err)
	}
	param := kernel.Param{
		KernelID:  os.Args[1],
		Services:  srvs,
		Dbip:      os.Args[4],
		Mongoip:   os.Args[5],
		DialProxy: dialproxy,
		BuildTag:  os.Args[7],
		Net:       os.Args[9],
		User:      os.Args[10],
	}
	if len(os.Args) >= 7 {
		param.ReserveMcpu = os.Args[6]
	}
	db.DPrintf(db.KERNEL, "param %v", param)
	h := sp.SIGMAHOME
	p := os.Getenv("PATH")
	os.Setenv("PATH", p+":"+h+"/bin/kernel:"+h+"/bin/linux:"+h+"/bin/user")
	localIP, err1 := iputil.LocalIP()
	if err1 != nil {
		db.DFatalf("Error local IP: %v", err1)
	}
	db.DPrintf(db.ALWAYS, "LocalIP: %v", localIP)
	s3secrets, err := auth.GetAWSSecrets(sp.AWS_PROFILE)
	if err != nil {
		db.DFatalf("Failed to load AWS secrets %v", err)
	}
	etcdMnt, err := fsetcd.NewFsEtcdEndpoint(sp.Tip(os.Args[2]))
	if err != nil {
		db.DFatalf("Error NewFsEtcdEndpoint: %v", err)
	}
	secrets := map[string]*sp.SecretProto{"s3": s3secrets}
	pe := proc.NewBootProcEnv(sp.NewPrincipal(sp.TprincipalID(param.KernelID), sp.ROOTREALM), secrets, etcdMnt, localIP, localIP, param.BuildTag)
	proc.SetSigmaDebugPid(pe.GetPID().String())
	if err := ini.BootUp(&param, pe); err != nil {
		db.DFatalf("%v: boot %v err %v", os.Args[0], os.Args[1:], err)
	}
	os.Exit(0)
}
