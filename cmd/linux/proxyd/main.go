package main

import (
	"log"
	"os"

	"sigmaos/config"
	"sigmaos/netsrv"
	"sigmaos/npcodec"
	"sigmaos/proc"
	"sigmaos/proxy"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("%s: Usage <lip>\n", os.Args[0])
	}
	scfg := config.NewTestSigmaConfig(sp.ROOTREALM, lip, lip, "")
	scfg.Program = "proxy"
	scfg.Uname = "proxy"
	proc.SetSigmaDebugPid(scfg.String())
	netsrv.MakeNetServer(scfg, proxy.MakeNpd(scfg, os.Args[1]), ":1110", npcodec.MarshalFrame, npcodec.UnmarshalFrame)
	ch := make(chan struct{})
	<-ch
}
