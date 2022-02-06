package main

import (
	"ulambda/netsrv"
	"ulambda/proc"
	"ulambda/proxy"
)

type Proxyd struct {
	done chan bool
	fsd  *proxy.Npd
	srv  *netsrv.NetServer
}

func makeProxyd() *Proxyd {
	pd := &Proxyd{}
	pd.done = make(chan bool)
	pd.fsd = proxy.MakeNpd()
	pd.srv = netsrv.MakeNetServerWireCompatible(":1110", pd.fsd)
	return pd
}

func main() {
	proc.SetProgram("proxy")
	pd := makeProxyd()
	<-pd.done
}
