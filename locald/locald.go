package locald

import (
	//	"github.com/sasha-s/go-deadlock"
	"encoding/json"
	"io"
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

const (
	NO_OP_LAMBDA = "no-op-lambda"
)

type LocalD struct {
	//	mu deadlock.Mutex
	mu   sync.Mutex
	cond *sync.Cond
	load int // XXX bogus
	bin  string
	nid  uint64
	root *Dir
	done bool
	ip   string
	ls   map[string]*Lambda
	srv  *npsrv.NpServer
	*fslib.FsLib
	group sync.WaitGroup
}

func MakeLocalD(bin string) *LocalD {
	ld := &LocalD{}
	ld.cond = sync.NewCond(&ld.mu)
	ld.load = 0
	ld.nid = 0
	ld.bin = bin
	db.Name("locald")
	ld.root = ld.makeDir([]string{}, np.DMDIR, nil)
	ld.root.time = time.Now().Unix()
	ld.ls = map[string]*Lambda{}
	ip, err := fsclnt.LocalIP()
	ld.ip = ip
	if err != nil {
		log.Fatalf("LocalIP %v\n", err)
	}
	ld.srv = npsrv.MakeNpServer(ld, ld.ip+":0")
	fsl := fslib.MakeFsLib("locald")
	fsl.Mkdir(fslib.LOCALD_ROOT, 0777)
	ld.FsLib = fsl
	err = fsl.PostServiceUnion(ld.srv.MyAddr(), fslib.LOCALD_ROOT, ld.srv.MyAddr())
	if err != nil {
		log.Fatalf("PostServiceUnion failed %v %v\n", ld.srv.MyAddr(), err)
	}
	// Try to make scheduling directories if they don't already exist
	fsl.Mkdir(fslib.SCHEDQ, 0777)
	fsl.Mkdir(fslib.LOCKS, 0777)
	return ld
}

func (ld *LocalD) spawn(a []byte) (*Lambda, error) {
	ld.mu.Lock()
	defer ld.mu.Unlock()
	l := &Lambda{}
	l.ld = ld
	err := l.init(a)
	if err != nil {
		return nil, err
	}
	ld.ls[l.Pid] = l
	return l, nil
}

func (ld *LocalD) Connect(conn net.Conn) npsrv.NpAPI {
	return npo.MakeNpConn(ld, conn)
}

func (ld *LocalD) Done() {
	ld.mu.Lock()
	defer ld.mu.Unlock()

	ld.done = true
	ld.SignalNewJob()
}

func (ld *LocalD) WatchTable() *npo.WatchTable {
	return nil
}

func (ld *LocalD) ConnTable() *npo.ConnTable {
	return nil
}

func (ld *LocalD) readDone() bool {
	ld.mu.Lock()
	defer ld.mu.Unlock()
	return ld.done
}

func (ld *LocalD) RootAttach(uname string) (npo.NpObj, npo.CtxI) {
	return ld.root, nil
}

// Tries to claim a job from the runq. If none are available, return.
func (ld *LocalD) getLambda() ([]byte, error) {
	err := ld.WaitForJob()
	if err != nil {
		return []byte{}, err
	}
	jobs, err := ld.ReadRunQ()
	if err != nil {
		return []byte{}, err
	}
	for _, j := range jobs {
		b, claimed := ld.ClaimJob(j.Name)
		if err != nil {
			return []byte{}, err
		}
		if claimed {
			return b, nil
		}
	}
	return []byte{}, nil
}

// Scan through the waitq, and try to move jobs to the runq.
func (ld *LocalD) checkWaitingLambdas() {
	jobs, err := ld.ReadWaitQ()
	if err != nil {
		log.Fatalf("Error reading WaitQ: %v", err)
	}
	for _, j := range jobs {
		b, err := ld.ReadWaitQJob(j.Name)
		// Ignore errors: they may be frequent under high concurrency
		if err != nil || len(b) == 0 {
			continue
		}
		if ld.jobIsRunnable(j, b) {
			// Ignore errors: they may be frequent under high concurrency
			ld.MarkJobRunnable(j.Name)
		}
	}
}

// TODO: Handle consumer/producer deps
func (ld *LocalD) jobIsRunnable(j *np.Stat, a []byte) bool {
	var attr fslib.Attr
	err := json.Unmarshal(a, &attr)
	if err != nil {
		log.Printf("Couldn't unmarshal job to check if runnable %v: %v", a, err)
		return false
	}
	// If this job is meant to run on a timer, and the timer has expired
	if attr.Timer != 0 {
		if uint32(time.Now().Unix()) > j.Mtime+attr.Timer {
			return true
		} else {
			// XXX Factor this out & do it in a monitor lambda
			// For now, just make sure *some* locald eventually wakes up to mark this
			// lambda as runnable
			go func(timer uint32) {
				dur := time.Duration(uint64(timer) * 1000000000)
				time.Sleep(dur)
				ld.SignalNewJob()
			}(attr.Timer)
			return false
		}
	}
	for _, b := range attr.ExitDep {
		if !b {
			return false
		}
	}
	return true
}

// Worker runs one lambda at a time
func (ld *LocalD) Worker() {
	ld.SignalNewJob()

	// TODO pin to a core
	for !ld.readDone() {
		b, err := ld.getLambda()
		// If no job was on the runq, try and move some from waitq -> runq
		if err == nil && len(b) == 0 {
			ld.checkWaitingLambdas()
			continue
		}
		if err == io.EOF {
			continue
		}
		if err != nil {
			log.Fatalf("Locald GetLambda error %v, %v\n", err, b)
		}
		// XXX return err from spawn
		l, err := ld.spawn(b)
		if err != nil {
			log.Fatalf("Locald spawn error %v\n", err)
		}
		l.run()
	}
	ld.SignalNewJob()
	ld.group.Done()
}

func (ld *LocalD) Work() {
	var NWorkers uint
	if NCores < 20 {
		NWorkers = 20
	} else {
		NWorkers = NCores
	}
	for i := uint(0); i < NWorkers; i++ {
		ld.group.Add(1)
		go ld.Worker()
	}
	ld.group.Wait()

}
