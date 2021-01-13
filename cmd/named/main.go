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

func makeNamed(debug bool) *Named {
	nd := &Named{}
	nd.done = make(chan bool)
	nd.fsd = memfsd.MakeFsd(debug)
	nd.srv = npsrv.MakeNpServer(nd.fsd, ":1111", false)
	return nd
}

func main() {
	nd := makeNamed(false)
	<-nd.done
}
