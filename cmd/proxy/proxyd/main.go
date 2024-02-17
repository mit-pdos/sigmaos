package main

import (
	"log"
	"os"

	"sigmaos/netsrv"
	"sigmaos/proc"
	"sigmaos/proxy"
	sp "sigmaos/sigmap"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("%s: Usage <lip>\n", os.Args[0])
	}
	lip := sp.Tip(os.Args[1])
	// By default, proxy doesn't use overlays.
	pcfg := proc.NewTestProcEnv(sp.ROOTREALM, lip, lip, lip, "", false, false)
	pcfg.Program = "proxy"
	pcfg.SetUname("proxy")
	addr := sp.NewTaddr(sp.NO_IP, sp.INNER_CONTAINER_IP, 1110)
	proc.SetSigmaDebugPid(pcfg.GetPID().String())
	npd := proxy.NewNpd(pcfg, lip)
	netsrv.NewNetServer(pcfg, addr, npd)
	ch := make(chan struct{})
	<-ch
}
