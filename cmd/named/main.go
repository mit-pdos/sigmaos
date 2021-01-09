package main

import (
	"log"

	"ulambda/memfsd"
	np "ulambda/ninep"
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
	nd.srv = npsrv.MakeNpServer(nd.fsd, ":1111")
	return nd
}

func (nd *Named) maketestfs() {
	fs := nd.fsd.Root()
	rooti := fs.RootInode()
	_, err := rooti.Create(fs, np.DMDIR|07000, "todo")
	if err != nil {
		log.Fatal("Create error ", err)
	}
	is, _, err := rooti.Walk([]string{"todo"})
	if err != nil {
		log.Fatal("Walk error ", err)
	}
	_, err = is[1].Create(fs, 07000, "job1")
	if err != nil {
		log.Fatal("Create error ", err)
	}
	_, err = rooti.Create(fs, np.DMDIR|07000, "started")
	if err != nil {
		log.Fatal("Create error ", err)
	}
	_, err = rooti.Create(fs, np.DMDIR|07000, "reduce")
	if err != nil {
		log.Fatal("Create error ", err)
	}
}

func main() {
	nd := makeNamed()
	nd.maketestfs()
	<-nd.done
}
