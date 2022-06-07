package main

import (
	"ulambda/netsrv"
	"ulambda/proc"
	"ulambda/proxy"
)

func main() {
	proc.SetProgram("proxy")
	netsrv.MakeNetServerWireCompatible(":1110", proxy.MakeNpd())
	ch := make(chan struct{})
	<-ch
}
