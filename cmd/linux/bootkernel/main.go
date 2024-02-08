package main

import (
	"os"
	"strconv"
	"strings"

	"sigmaos/auth"
	"sigmaos/boot"
	db "sigmaos/debug"
	"sigmaos/kernel"
	"sigmaos/netsigma"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

func main() {
	if len(os.Args) < 11 {
		db.DFatalf("usage: %v kernelid srvs nameds dbip mongoip overlays reserveMcpu buildTag gvisor key\nprovided:%v", os.Args[0], os.Args)
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
	masterKey := os.Args[10]
	param := kernel.Param{
		MasterKey: auth.SymmetricKey(masterKey),
		KernelId:  os.Args[1],
		Services:  srvs,
		Dbip:      os.Args[4],
		Mongoip:   os.Args[5],
		Overlays:  overlays,
		BuildTag:  os.Args[8],
		GVisor:    gvisor,
	}
	if len(os.Args) >= 8 {
		param.ReserveMcpu = os.Args[7]
	}
	db.DPrintf(db.KERNEL, "param %v\n", param)
	h := sp.SIGMAHOME
	p := os.Getenv("PATH")
	os.Setenv("PATH", p+":"+h+"/bin/kernel:"+h+"/bin/linux:"+h+"/bin/user")
	localIP, err1 := netsigma.LocalIP()
	if err1 != nil {
		db.DFatalf("Error local IP: %v", err1)
	}
	s3secrets, err := auth.GetAWSSecrets()
	if err != nil {
		db.DFatalf("Failed to load AWS secrets %v", err)
	}
	secrets := map[string]*proc.ProcSecretProto{"s3": s3secrets}
	pe := proc.NewBootProcEnv(sp.NewPrincipal(sp.TprincipalID(param.KernelId), sp.NO_TOKEN), secrets, sp.Tip(os.Args[2]), localIP, localIP, param.BuildTag, param.Overlays)
	proc.SetSigmaDebugPid(pe.GetPID().String())
	as, err1 := auth.NewHMACAuthSrv(proc.NOT_SET, []byte(masterKey))
	if err1 != nil {
		db.DFatalf("Error NewAuthSrv: %v", err1)
	}
	pc := auth.NewProcClaims(pe)
	token, err1 := as.NewToken(pc)
	if err1 != nil {
		db.DFatalf("Error NewToken: %v", err1)
	}
	pe.SetToken(token)
	if err := boot.BootUp(&param, pe, as); err != nil {
		db.DFatalf("%v: boot %v err %v\n", os.Args[0], os.Args[1:], err)
	}
	os.Exit(0)
}
