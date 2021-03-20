package locald

import (
	//	"github.com/sasha-s/go-deadlock"
	"log"
	"net"
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
	//	mu deadlock.Mutex
	mu   sync.Mutex
	cond *sync.Cond
	load int // XXX bogus
	bin  string
	nid  uint64
	root *Obj
	ip   string
	ls   map[string]*Lambda
	srv  *npsrv.NpServer
	*fslib.FsLib
	ch chan bool
}

func MakeLocalD(bin string) *LocalD {
	ld := &LocalD{}
	ld.cond = sync.NewCond(&ld.mu)
	ld.load = 0
	ld.nid = 0
	ld.bin = bin
	db.Name("locald")
	ld.root = ld.MakeObj([]string{}, np.DMDIR, nil).(*Obj)
	ld.root.time = time.Now().Unix()
	ld.ls = map[string]*Lambda{}
	ld.ch = make(chan bool)
	ip, err := fsclnt.LocalIP()
	ld.ip = ip
	if err != nil {
		log.Fatalf("LocalIP %v %v\n", fslib.SCHED, err)
	}
	ld.srv = npsrv.MakeNpServer(ld, ld.ip+":0")
	fsl := fslib.MakeFsLib("locald")
	fsl.Mkdir(fslib.LOCALD_ROOT, 0777)
	ld.FsLib = fsl
	err = fsl.PostServiceUnion(ld.srv.MyAddr(), fslib.LOCALD_ROOT, ld.srv.MyAddr())
	if err != nil {
		log.Fatalf("PostServiceUnion failed %v %v\n", ld.srv.MyAddr(), err)
	}
	return ld
}

func (ld *LocalD) spawn(a []byte) error {
	ld.mu.Lock()
	defer ld.mu.Unlock()
	l := &Lambda{}
	l.ld = ld
	err := l.init(a)
	if err != nil {
		return err
	}
	ld.ls[l.Pid] = l
	l.run()
	return nil
}

func (ld *LocalD) Connect(conn net.Conn) npsrv.NpAPI {
	return npo.MakeNpConn(ld, conn)
}

func (ld *LocalD) Done() {
	ld.ch <- true
}

func (ld *LocalD) Root() npo.NpObj {
	return ld.root
}

func (ld *LocalD) Resolver() npo.Resolver {
	return nil
}

func (ld *LocalD) Work() {
	<-ld.ch
}
