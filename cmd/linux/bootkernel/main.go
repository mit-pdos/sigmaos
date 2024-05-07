package main

import (
	"os"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt"

	"sigmaos/auth"
	"sigmaos/boot"
	db "sigmaos/debug"
	"sigmaos/fsetcd"
	"sigmaos/kernel"
	"sigmaos/keys"
	"sigmaos/netsigma"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

func main() {
	if len(os.Args) != 13 {
		db.DFatalf("usage: %v kernelid srvs nameds dbip mongoip overlays reserveMcpu buildTag gvisor netproxy pubkey privkeyprovided:%v", os.Args[0], os.Args)
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
	masterPubKey, err := auth.NewPublicKey[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, []byte(os.Args[11]))
	if err != nil {
		db.DFatalf("Error NewPublicKey", err)
	}
	masterPrivKey, err := auth.NewPrivateKey[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, []byte(os.Args[12]))
	if err != nil {
		db.DFatalf("Error NewPrivateKey", err)
	}
	param := kernel.Param{
		MasterPubKey:  masterPubKey,
		MasterPrivKey: masterPrivKey,
		KernelID:      os.Args[1],
		Services:      srvs,
		Dbip:          os.Args[4],
		Mongoip:       os.Args[5],
		Overlays:      overlays,
		NetProxy:      netproxy,
		BuildTag:      os.Args[8],
		GVisor:        gvisor,
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
	// Create an auth server with a constant GetKeyFn, to bootstrap with the
	// initial master key. This auth server should *not* be used long-term. It
	// needs to be replaced with one which queries the namespace for keys once
	// knamed has booted.
	kmgr := keys.NewKeyMgrWithBootstrappedKeys(
		keys.WithConstGetKeyFn(auth.PublicKey(masterPubKey)),
		masterPubKey,
		masterPrivKey,
		auth.SIGMA_DEPLOYMENT_MASTER_SIGNER,
		masterPubKey,
		masterPrivKey,
	)
	amgr, err1 := auth.NewAuthMgr[*jwt.SigningMethodECDSA](jwt.SigningMethodES256, auth.SIGMA_DEPLOYMENT_MASTER_SIGNER, sp.NOT_SET, kmgr)
	if err1 != nil {
		db.DFatalf("Error NewAuthMgr: %v", err1)
	}
	etcdMnt, err := fsetcd.NewFsEtcdEndpoint(amgr, sp.Tip(os.Args[2]))
	if err != nil {
		db.DFatalf("Error NewFsEtcdEndpoint: %v", err)
	}
	secrets := map[string]*proc.ProcSecretProto{"s3": s3secrets}
	// Only verify mounts if using netproxy
	verifyEndpoints := param.NetProxy
	pe := proc.NewBootProcEnv(sp.NewPrincipal(sp.TprincipalID(param.KernelID), sp.ROOTREALM, sp.NoToken()), secrets, etcdMnt, localIP, localIP, param.BuildTag, param.Overlays, verifyEndpoints)
	proc.SetSigmaDebugPid(pe.GetPID().String())
	if err1 := amgr.MintAndSetProcToken(pe); err1 != nil {
		db.DFatalf("Error MintToken: %v", err1)
	}
	if err := boot.BootUp(&param, pe, amgr); err != nil {
		db.DFatalf("%v: boot %v err %v", os.Args[0], os.Args[1:], err)
	}
	os.Exit(0)
}
