package main

import (
	"io"
	"log"
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
}

func makeCntlr() *Cntlr {
	cr := &Cntlr{}
	cr.clnt = fsclnt.MakeFsClient("cntlr", false)
	cr.fsd = memfsd.MakeFsd(false)
	cr.srv = npsrv.MakeNpServer(cr.fsd, ":0", false)
	return cr
}

func (cr *Cntlr) isEmpty(name string) bool {
	st, err := cr.clnt.Stat(name)
	if err != nil {
		log.Fatalf("Stat %v error %v\n", name, err)
	}
	return st.Length == 0
}

func (cr *Cntlr) check() {
	log.Print("check")
	fd, err := cr.clnt.Opendir("name/mr/started")
	if err != nil {
		log.Fatal("Opendir error ", err)
	}
	for {
		dirents, err := cr.clnt.Readdir(fd, 1024)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("Readdir %v\n", err)
		}
		for _, st := range dirents {
			log.Printf("in progress: %v\n", st.Name)
			timeout := int64(st.Mtime) + 5
			if timeout < time.Now().Unix() {
				log.Print("REDO ", st.Name)
				err = cr.clnt.Rename("name/mr/started/"+st.Name,
					"name/mr/todo/"+st.Name)
			}
		}
	}
	cr.clnt.Close(fd)
}

func (cr *Cntlr) monitor() {
	for !cr.isEmpty("name/mr/todo/") || !cr.isEmpty("name/mr/started/") {
		time.Sleep(time.Duration(1000) * time.Millisecond)
		cr.check()
	}
}

func (cr *Cntlr) initfs() {
	fs := cr.fsd.Root()
	rooti := fs.RootInode()
	_, err := rooti.Create(0, fs, np.DMDIR|07000, "todo")
	if err != nil {
		log.Fatal("Create error ", err)
	}
	//is, _, err := rooti.Walk(0, []string{"todo"})
	//if err != nil {
	//	log.Fatal("Walk error ", err)
	//}

	// input directory

	// for i := 0; i < 5; i++ {
	// 	_, err = is[1].Create(0, fs, 07000, "job"+strconv.Itoa(i))
	// 	if err != nil {
	// 		log.Fatal("Create error ", err)
	// 	}
	// }
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
	for {
		cr.monitor()
	}
	log.Printf("Cntlr: finished\n")
}
