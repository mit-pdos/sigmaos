package main

import (
	"ulambda/netsrv"
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
	pd.srv = netsrv.MakeNpServerWireCompatible(":1110", pd.fsd)
	return pd
}

func main() {
	pd := makeProxyd()
	<-pd.done
}
