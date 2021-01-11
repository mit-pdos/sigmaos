package main

import (
	"ulambda/memfsd"
	"ulambda/npsrv"
)

type Named struct {
	done chan bool
	fsd  *memfsd.Fsd
	srv  *npsrv.NpServer
}

func makeNamed() *Named {
	nd := &Named{}
	nd.done = make(chan bool)
	nd.fsd = memfsd.MakeFsd()
	nd.srv = npsrv.MakeNpServer(nd.fsd, ":1111", true)
	return nd
}

func main() {
	nd := makeNamed()
	<-nd.done
}
