package locald

import (
	//	"github.com/sasha-s/go-deadlock"
	"log"
	"net"
	"os"
	"strconv"
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
	ch   chan bool
	name string
}

func MakeLocalD(bin string) *LocalD {
	ld := &LocalD{}
	ld.cond = sync.NewCond(&ld.mu)
	ld.load = 0
	ld.nid = 0
	ld.bin = bin
	ld.root = ld.MakeObj([]string{}, np.DMDIR, nil).(*Obj)
	ld.root.time = time.Now().Unix()
	ld.ls = map[string]*Lambda{}
	ld.ch = make(chan bool)
	ld.name = "locald:" + strconv.Itoa(os.Getpid())
	db.SetDebug(false)
	ip, err := fsclnt.LocalIP()
	ld.ip = ip
	if err != nil {
		log.Fatalf("LocalIP %v %v\n", fslib.SCHED, err)
	}
	ld.srv = npsrv.MakeNpServer(ld, ld.name, ld.ip+":0")
	fsl := fslib.MakeFsLib(ld.name)
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
	err := l.init(a)
	if err != nil {
		return err
	}
	l.ld = ld
	ld.ls[l.Pid] = l
	l.run()
	return nil
}

func (ld *LocalD) Connect(conn net.Conn) npsrv.NpAPI {
	return npo.MakeNpConn(ld, conn, ld.name)
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
