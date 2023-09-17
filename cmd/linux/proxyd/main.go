package main

import (
	"log"
	"os"

	"sigmaos/netsrv"
	"sigmaos/npcodec"
	"sigmaos/proc"
	"sigmaos/proxy"
	sp "sigmaos/sigmap"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("%s: Usage <lip>\n", os.Args[0])
	}
	lip := os.Args[1]
	// By default, proxy doesn't use overlays.
	pcfg := proc.NewTestProcEnv(sp.ROOTREALM, lip, lip, "", false)
	pcfg.Program = "proxy"
	pcfg.SetUname("proxy")
	proc.SetSigmaDebugPid(pcfg.String())
	netsrv.NewNetServer(pcfg, proxy.NewNpd(pcfg, lip), ":1110", npcodec.MarshalFrame, npcodec.UnmarshalFrame)
	ch := make(chan struct{})
	<-ch
}
