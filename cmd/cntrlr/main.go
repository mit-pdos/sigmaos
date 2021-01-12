package main

import (
	"io"
	"log"
	"strconv"
	"time"

	"ulambda/fsclnt"
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

func (cr *Cntlr) check() bool {
	log.Print("check")
	fd, err := cr.clnt.Opendir("name/mr/started")
	if err != nil {
		log.Fatal("Opendir error ", err)
	}
	done := false
	for {
		dirents, err := cr.clnt.Readdir(fd, 1024)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Readdir %v\n", err)
		}
		for _, st := range dirents {
			mtime := time.Unix(int64(st.Mtime), 0)
			log.Printf("st Name %v mtime %v sz %v\n", st.Name, mtime)
			mtime.Add(time.Duration(5 * time.Second))
			if mtime.After(time.Now()) {
				log.Print("redo")
				// mov from started to todo
			}
		}
	}
	cr.clnt.Close(fd)
	return done
}

func (cr *Cntlr) monitor() {
	done := false
	for !done {
		time.Sleep(time.Duration(1000) * time.Millisecond)
		done = cr.check()
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
	for i := 0; i < 5; i++ {
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
		err = cr.clnt.Remove("name/mr")
		if err != nil {
			log.Print("remove name/mr failed")
		}
		name := cr.srv.MyAddr()
		err = cr.clnt.Symlink(name+":pubkey:", "name/mr", 0777)
		if err != nil {
			log.Fatal("Symlink error: ", err)
		}
	}
	cr.monitor()
	<-cr.done
	// cr.clnt.Close(fd)
	log.Printf("Cntlr: finished\n")
}
