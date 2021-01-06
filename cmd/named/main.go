package main

import (
	"ulambda/fsd"
	"ulambda/npsrv"
)

type Named struct {
	done chan bool
	fsd  *fsd.Fsd
	srv  *npsrv.NpServer
}

func makeNamed() *Named {
	nd := &Named{}
	nd.done = make(chan bool)
	nd.fsd = fsd.MakeFsd()
	nd.srv = npsrv.MakeNpServer(nd.fsd, ":1111")
	return nd
}

func main() {
	nd := makeNamed()
	<-nd.done
}
