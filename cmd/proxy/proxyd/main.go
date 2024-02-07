package main

import (
	"os"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/netsrv"
	"sigmaos/npcodec"
	"sigmaos/proc"
	"sigmaos/proxy"
	sp "sigmaos/sigmap"
)

func main() {
	if len(os.Args) < 2 {
		db.DFatalf("%s: Usage <lip>\n", os.Args[0])
	}
	lip := sp.Tip(os.Args[1])
	s3secrets, err1 := auth.GetAWSSecrets()
	if err1 != nil {
		db.DFatalf("Failed to load AWS secrets %v", err1)
	}
	secrets := map[string]*proc.ProcSecretProto{"s3": s3secrets}
	// By default, proxy doesn't use overlays.
	pcfg := proc.NewTestProcEnv(sp.ROOTREALM, secrets, lip, lip, lip, "", false, false)
	pcfg.Program = "proxy"
	pcfg.SetPrincipal(&sp.Tprincipal{
		ID:       "proxy",
		TokenStr: proc.NOT_SET,
	})
	addr := sp.NewTaddr(sp.NO_IP, sp.INNER_CONTAINER_IP, 1110)
	proc.SetSigmaDebugPid(pcfg.GetPID().String())
	netsrv.NewNetServer(pcfg, proxy.NewNpd(pcfg, lip), addr, npcodec.ReadCall, npcodec.WriteCall)
	ch := make(chan struct{})
	<-ch
}
