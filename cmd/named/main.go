package main

import (
	"ulambda/fsd"
	"ulambda/fssrv"
)

type Named struct {
	done chan bool
	fsd  *fsd.Fsd
	srv  *fssrv.FsServer
}

func makeNamed() *Named {
	nd := &Named{}
	nd.done = make(chan bool)
	nd.fsd = fsd.MakeFsd()
	nd.srv = fssrv.MakeFsServer(nd.fsd, ":1111")
	return nd
}

func main() {
	nd := makeNamed()
	<-nd.done
}
