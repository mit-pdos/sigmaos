package main

import (
	"sigmaos/netsrv"
	"sigmaos/proc"
	"sigmaos/proxy"
)

func main() {
	proc.SetProgram("proxy")
	netsrv.MakeNetServer9p(":1110", proxy.MakeNpd())
	ch := make(chan struct{})
	<-ch
}
