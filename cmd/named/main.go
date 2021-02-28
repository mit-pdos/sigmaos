package main

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"ulambda/fslib"
	"ulambda/memfs"
	"ulambda/memfsd"
	np "ulambda/ninep"
	"ulambda/npsrv"
)

type Dev struct {
	nd *Named
}

func (dev *Dev) Write(off np.Toffset, data []byte) (np.Tsize, error) {
	t := string(data)
	if strings.HasPrefix(t, "Exit") {
		dev.nd.done <- true
	} else {
		return 0, fmt.Errorf("Write: unknown command %v\n", t)
	}
	return np.Tsize(len(data)), nil
}

func (dev *Dev) Read(off np.Toffset, n np.Tsize) ([]byte, error) {
	return nil, errors.New("Not support")
}

func (dev *Dev) Len() np.Tlength { return 0 }

type Named struct {
	done chan bool
	fsd  *memfsd.Fsd
	srv  *npsrv.NpServer
}

func makeNamed() *Named {
	nd := &Named{}
	nd.done = make(chan bool)
	// Because named is a bit special, we don't use InitFS
	memfs := memfs.MakeRoot()
	_, err := memfs.MkNod("named", memfs.RootInode(), "dev", &Dev{nd})
	if err != nil {
		log.Fatal("Create error: dev: ", err)
	}
	nd.fsd = memfsd.MakeFsd(memfs, nil)
	nd.srv = npsrv.MakeNpServer(nd.fsd, fslib.Named())
	return nd
}

func main() {
	nd := makeNamed()
	<-nd.done
}
