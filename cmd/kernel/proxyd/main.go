package main

import (
	db "ulambda/debug"
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
	pd.srv = netsrv.MakeNetServerWireCompatible(":1110", pd.fsd)
	return pd
}

func main() {
	db.Name("proxy")
	pd := makeProxyd()
	<-pd.done
}
