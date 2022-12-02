package main

import (
	"sigmaos/netsrv"
	"sigmaos/npcodec"
	"sigmaos/proc"
	"sigmaos/proxy"
)

func main() {
	proc.SetProgram("proxy")
	netsrv.MakeNetServer(proxy.MakeNpd(), ":1110", npcodec.MarshalFcallMsg, npcodec.UnmarshalFcallWireCompat)
	ch := make(chan struct{})
	<-ch
}
