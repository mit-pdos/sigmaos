package main

import (
	"os"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/fsetcd"
	"sigmaos/netproxyclnt"
	"sigmaos/netsrv"
	"sigmaos/proc"
	"sigmaos/proxy"
	sp "sigmaos/sigmap"
)

func main() {
	if len(os.Args) < 2 {
		db.DFatalf("%s: Usage <lip>\n", os.Args[0])
	}
	proc.SetSigmaDebugPid("proxy")
	lip := sp.Tip(os.Args[1])
	s3secrets, err1 := auth.GetAWSSecrets(sp.AWS_PROFILE)
	if err1 != nil {
		db.DFatalf("Failed to load AWS secrets %v", err1)
	}
	secrets := map[string]*proc.ProcSecretProto{"s3": s3secrets}
	etcdMnt, err := fsetcd.NewFsEtcdEndpoint(lip)
	if err != nil {
		db.DFatalf("Error new fsetcd moutn: %v", err)
	}
	// By default, proxy doesn't use overlays.
	pe := proc.NewTestProcEnv(sp.ROOTREALM, secrets, etcdMnt, lip, lip, "", false, false, false, false)
	pe.SetPID("proxy")
	pe.Program = "proxy"
	pe.SetPrincipal(sp.NewPrincipal(
		sp.TprincipalID("proxy"),
		sp.ROOTREALM,
		sp.NoToken(),
	))
	addr := sp.NewTaddr(sp.NO_IP, sp.INNER_CONTAINER_IP, 1110)
	npc := netproxyclnt.NewNetProxyClnt(pe)
	npd := proxy.NewNpd(pe, npc, lip)
	netsrv.NewNetServer(pe, npc, addr, npd)
	ch := make(chan struct{})
	<-ch
}
