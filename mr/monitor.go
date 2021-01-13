package mr

import (
	"io"
	"log"
	"time"

	"ulambda/fsclnt"
	"ulambda/memfsd"
	np "ulambda/ninep"
	"ulambda/npsrv"
)

type Mond struct {
	clnt *fsclnt.FsClient
	srv  *npsrv.NpServer
	fsd  *memfsd.Fsd
}

// XXX really should be shell script
func (md *Mond) initfs() {
	fs := md.fsd.Root()
	rooti := fs.RootInode()
	_, err := rooti.Create(0, fs, np.DMDIR|07000, "todo")
	if err != nil {
		log.Fatal("Create error ", err)
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

func MakeMond() *Mond {
	md := &Mond{}
	md.clnt = fsclnt.MakeFsClient("cntlr", false)
	md.fsd = memfsd.MakeFsd(false)
	md.srv = npsrv.MakeNpServer(md.fsd, ":0", false)
	if fd, err := md.clnt.Attach(":1111", ""); err == nil {
		err := md.clnt.Mount(fd, "name")
		if err != nil {
			log.Fatal("Mount error: ", err)
		}
		err = md.clnt.Remove("name/mr")
		if err != nil {
			log.Print("remove name/mr failed")
		}
		name := md.srv.MyAddr()
		err = md.clnt.Symlink(name+":pubkey:", "name/mr", 0777)
		if err != nil {
			log.Fatal("Symlink error: ", err)
		}
		md.initfs()
	} else {
		log.Fatal("Attach failed error: ", err)
	}

	return md
}

func (md *Mond) isEmpty(name string) bool {
	st, err := md.clnt.Stat(name)
	if err != nil {
		log.Fatalf("Stat %v error %v\n", name, err)
	}
	return st.Length == 0
}

func (md *Mond) check() {
	log.Print("check")
	fd, err := md.clnt.Opendir("name/mr/started")
	if err != nil {
		log.Fatal("Opendir error ", err)
	}
	for {
		dirents, err := md.clnt.Readdir(fd, 1024)
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
				err = md.clnt.Rename("name/mr/started/"+st.Name,
					"name/mr/todo/"+st.Name)
			}
		}
	}
	md.clnt.Close(fd)
}

func (md *Mond) Monitor() {
	time.Sleep(time.Duration(1000) * time.Millisecond)
	for !md.isEmpty("name/mr/started/") {
		md.check()
	}
}
