package locald

import (
	//	"github.com/sasha-s/go-deadlock"
	"log"
	"net"
	"path"
	"sync"
	"time"

	db "ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslib"
	np "ulambda/ninep"
	npo "ulambda/npobjsrv"
	"ulambda/npsrv"
)

type LocalD struct {
	mu   sync.Mutex
	load int // XXX bogus
	nid  uint64
	root *Obj
	ip   string
	done bool
	srv  *npsrv.NpServer
}

func MakeLocalD() *LocalD {
	ld := &LocalD{}
	ld.load = 0
	ld.nid = 0
	ld.root = ld.MakeObj([]string{}, np.DMDIR, nil).(*Obj)
	ld.root.time = time.Now().Unix()
	db.SetDebug(false)
	ip, err := fsclnt.LocalIP()
	ld.ip = ip
	if err != nil {
		log.Fatalf("LocalIP %v %v\n", fslib.SCHED, err)
	}
	ld.srv = npsrv.MakeNpServer(ld, ld.ip+":0")
	fsl := fslib.MakeFsLib("locald")
	err = fsl.PostService(ld.srv.MyAddr(), path.Join(fslib.LOCALD_ROOT, ld.ip)) //"~"+ld.ip))
	if err != nil {
		log.Fatalf("PostService failed %v %v\n", fslib.LOCALD_ROOT, err)
	}
	return ld
}

func (ld *LocalD) Connect(conn net.Conn) npsrv.NpAPI {
	return npo.MakeNpConn(ld, conn)
}

func (ld *LocalD) Done() {
	ld.mu.Lock()
	defer ld.mu.Unlock()

	ld.done = true
}

func (ld *LocalD) Root() npo.NpObj {
	return ld.root
}

func (ld *LocalD) Resolver() npo.Resolver {
	return nil
}

func (ld *LocalD) Work() {
	for {
	}
}
