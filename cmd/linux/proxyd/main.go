package main

import (
	"log"
	"os"

	"sigmaos/config"
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
	scfg := config.NewTestProcEnv(sp.ROOTREALM, lip, lip, "")
	scfg.Program = "proxy"
	scfg.Uname = "proxy"
	proc.SetSigmaDebugPid(scfg.String())
	netsrv.MakeNetServer(scfg, proxy.MakeNpd(scfg, lip), ":1110", npcodec.MarshalFrame, npcodec.UnmarshalFrame)
	ch := make(chan struct{})
	<-ch
}
