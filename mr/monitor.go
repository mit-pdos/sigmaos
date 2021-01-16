package mr

import (
	"log"
	"strconv"
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

// XXX could be a shell script
func (md *Mond) initfs() {
	fs := md.fsd.Root()
	rooti := fs.RootInode()
	_, err := rooti.Create(0, fs, np.DMDIR|07000, "map")
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
	// XXX have a create in memfs that combines walk and Create?
	is, _, err := rooti.Walk(0, []string{"reduce"})
	if err != nil {
		log.Fatal("Walk error ", err)
	}
	for r := 0; r < NReduce; r++ {
		_, err = is[1].Create(0, fs, np.DMDIR|07000, strconv.Itoa(r))
		if err != nil {
			log.Fatal("Create error ", err)
		}
	}
}

func MakeMond() *Mond {
	md := &Mond{}
	md.clnt = fsclnt.MakeFsClient(false)
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

// log.Printf("in progress: %v\n", st.Name)
// timeout := int64(st.Mtime) + 5
// if timeout < time.Now().Unix() {
// 	log.Print("REDO ", st.Name)
// 	err = md.clnt.Rename("name/mr/started/"+st.Name,
// 		"name/mr/todo/"+st.Name)
// }

func (md *Mond) Monitor() {

	time.Sleep(time.Duration(1000) * time.Millisecond)
	//for !md.isEmpty("name/mr/started/") {
	// XXX check could be a lambda?
	// md.check()
	//}
}
