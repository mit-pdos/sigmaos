package main

import (
	"ulambda/npsrv"
	"ulambda/proxy"
)

type Proxyd struct {
	done chan bool
	fsd  *proxy.Npd
	srv  *npsrv.NpServer
}

func makeProxyd() *Proxyd {
	pd := &Proxyd{}
	pd.done = make(chan bool)
	pd.fsd = proxy.MakeNpd()
	pd.srv = npsrv.MakeNpServerWireCompatible(":1110", pd.fsd)
	return pd
}

func main() {
	pd := makeProxyd()
	<-pd.done
}
