package main

import (
	"ulambda/memfs"
	"ulambda/memfsd"
	"ulambda/npsrv"
)

type Named struct {
	done chan bool
	fsd  *memfsd.Fsd
	srv  *npsrv.NpServer
}

func makeNamed(debug bool) *Named {
	memfs := memfs.MakeRoot(false)
	nd := &Named{}
	nd.done = make(chan bool)
	nd.fsd = memfsd.MakeFsd(debug, memfs, nil, nil)
	nd.srv = npsrv.MakeNpServer(nd.fsd, ":1111", debug)
	return nd
}

func main() {
	nd := makeNamed(false)
	<-nd.done
}
