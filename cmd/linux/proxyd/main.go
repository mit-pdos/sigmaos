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
	if len(os.Args) < 3 {
		log.Fatalf("%s: Usage <lip> <namedaddr>...\n", os.Args[0])
	}
	proc.SetProgram("proxy")
	netsrv.MakeNetServer(proxy.MakeNpd(os.Args[1], sp.MkTaddrs(os.Args[2:])), ":1110", npcodec.MarshalFrame, npcodec.UnmarshalFrame)
	ch := make(chan struct{})
	<-ch
}
