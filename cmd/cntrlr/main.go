package main

import (
	"log"
	"strconv"

	"ulambda/fsclnt"
	// "ulambda/memfs"
	"ulambda/memfsd"
	np "ulambda/ninep"
	"ulambda/npsrv"
)

const (
	NReduce = 1
)

type Cntlr struct {
	clnt *fsclnt.FsClient
	srv  *npsrv.NpServer
	fsd  *memfsd.Fsd
	done chan bool
}

func makeCntlr() *Cntlr {
	cr := &Cntlr{}
	cr.clnt = fsclnt.MakeFsClient("cntlr", false)
	cr.fsd = memfsd.MakeFsd()
	cr.srv = npsrv.MakeNpServer(cr.fsd, ":0", false)
	cr.done = make(chan bool)
	return cr
}

func pickOld(dirents []np.Stat) (string, bool) {
	// look at time
	return dirents[0].Name, true
}

func (cr *Cntlr) Monitor() {
	done := false
	for !done {
		fd, err := cr.clnt.Opendir("name/todo")
		if err != nil {
			log.Fatal("Opendir error ", err)
		}
		dirents, err := cr.clnt.Readdir(fd, 0, 256)
		if err != nil {
			log.Fatal("Readdir error ", err)
		}
		cr.clnt.Close(fd)
		if len(dirents) == 0 {
			done = true
		} else {
			// name, ok := pickOld(dirents)
			//if ok {
			// XXX move back to started
			//}
			// spin until done
			// XXX maybe read from a named pipe
		}
	}
}

func (cr *Cntlr) initfs() {
	fs := cr.fsd.Root()
	rooti := fs.RootInode()
	_, err := rooti.Create(0, fs, np.DMDIR|07000, "todo")
	if err != nil {
		log.Fatal("Create error ", err)
	}
	is, _, err := rooti.Walk(0, []string{"todo"})
	if err != nil {
		log.Fatal("Walk error ", err)
	}
	for i := 0; i < 100; i++ {
		_, err = is[1].Create(0, fs, 07000, "job"+strconv.Itoa(i))
		if err != nil {
			log.Fatal("Create error ", err)
		}
	}
	_, err = rooti.Create(0, fs, np.DMDIR|07000, "started")
	if err != nil {
		log.Fatal("Create error ", err)
	}
	_, err = rooti.Create(0, fs, np.DMDIR|07000, "reduce")
	if err != nil {
		log.Fatal("Create error ", err)
	}
}

func main() {
	cr := makeCntlr()
	cr.initfs()
	if fd, err := cr.clnt.Attach(":1111", ""); err == nil {
		err := cr.clnt.Mount(fd, "name")
		if err != nil {
			log.Fatal("Mount error: ", err)
		}
		name := cr.srv.MyAddr()
		err = cr.clnt.Symlink(name+":pubkey:", "name/mr", 0777)
		if err != nil {
			log.Fatal("Symlink error: ", err)
		}
	}
	<-cr.done
	// cr.clnt.Close(fd)
	log.Printf("Cntlr: finished\n")
}
