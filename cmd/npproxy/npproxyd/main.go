package main

import (
	"os"

	"sigmaos/auth"
	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/namesrv/fsetcd"
	netsrv "sigmaos/net/srv"
	"sigmaos/proxy/ninep"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

func main() {
	if len(os.Args) < 2 {
		db.DFatalf("%s: Usage <lip>\n", os.Args[0])
	}
	proc.SetSigmaDebugPid("npproxyd")
	lip := sp.Tip(os.Args[1])
	s3secrets, err1 := auth.GetAWSSecrets(sp.AWS_PROFILE)
	if err1 != nil {
		db.DFatalf("Failed to load AWS secrets %v", err1)
	}
	secrets := map[string]*sp.SecretProto{"s3": s3secrets}
	etcdMnt, err := fsetcd.NewFsEtcdEndpoint(lip)
	if err != nil {
		db.DFatalf("Error new fsetcd moutn: %v", err)
	}
	// By default, proxy doesn't use overlays.
	pe := proc.NewTestProcEnv(sp.ROOTREALM, secrets, etcdMnt, lip, lip, "", false, true)
	pe.SetPID("proxy")
	pe.Program = "proxy"
	pe.SetPrincipal(sp.NewPrincipal(
		sp.TprincipalID("proxy"),
		sp.ROOTREALM,
	))
	db.DPrintf(db.NPPROXY, "Proxy env: %v", pe)
	addr := sp.NewTaddr(sp.NO_IP, sp.INNER_CONTAINER_IP, 1110)
	npc := dialproxyclnt.NewDialProxyClnt(pe)
	npd := npproxysrv.NewNpd(pe, npc, lip)
	netsrv.NewNetServerEPType(pe, npc, addr, sp.EXTERNAL_EP, npd)
	ch := make(chan struct{})
	<-ch
}
