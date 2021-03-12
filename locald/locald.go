package locald

import (
	//	"github.com/sasha-s/go-deadlock"
	"encoding/json"
	"log"
	"net"
	"os"
	"os/exec"
	"path"
	"sync"
	"time"

	db "ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslib"
	np "ulambda/ninep"
	npo "ulambda/npobjsrv"
	"ulambda/npsrv"
	"ulambda/schedd"
)

type LocalD struct {
	mu   sync.Mutex
	load int // XXX bogus
	nid  uint64
	root *Obj
	ip   string
	done bool
	ls   map[string]*schedd.Lambda
	srv  *npsrv.NpServer
	*fslib.FsLib
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
	fsl.Mkdir(fslib.LOCALD_ROOT, 0777)
	ld.FsLib = fsl
	err = fsl.PostService(ld.srv.MyAddr(), path.Join(fslib.LOCALD_ROOT, ld.ip)) //"~"+ld.ip))
	if err != nil {
		log.Fatalf("PostService failed %v %v\n", fslib.LOCALD_ROOT, err)
	}
	return ld
}

func (ld *LocalD) wait(attr fslib.Attr, cmd *exec.Cmd) {
	err := cmd.Wait()
	if err != nil {
		log.Printf("Lambda %v finished with error: %v", attr, err)
		// XXX Need to think about how to return errors
		//		return err
	}

	// XXX Race condition in fslib requires this to be locked
	ld.mu.Lock()
	defer ld.mu.Unlock()

	// Notify schedd that the process exited
	ld.Exiting(attr.Pid, "OK")
}

func (ld *LocalD) spawn(a []byte) error {
	var attr fslib.Attr
	err := json.Unmarshal(a, &attr)
	if err != nil {
		log.Printf("Locald unmarshalling error\n: %v", err)
		return err
	}
	db.DPrintf("Locald spawn: %v\n", attr)
	args := append([]string{attr.Pid}, attr.Args...)
	env := append(os.Environ(), attr.Env...)
	cmd := exec.Command(attr.Program, args...)
	cmd.Env = env
	cmd.Dir = attr.Dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		log.Printf("Locald run error: %v, %v\n", attr, err)
		return err
	}

	go ld.wait(attr, cmd)
	return nil
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
