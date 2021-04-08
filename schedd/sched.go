package schedd

import (
	//	"github.com/sasha-s/go-deadlock"
	"encoding/json"
	"io"
	"log"
	"math/rand"
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

// XXX monitor, boost

const (
	NO_OP_LAMBDA      = "no-op-lambda"
	LOCALD_UNASSIGNED = "locald-unassigned"
)

type Sched struct {
	//	mu deadlock.Mutex
	mu   sync.Mutex
	cond *sync.Cond
	load int // XXX bogus
	nid  uint64
	ls   map[string]*Lambda
	root *Dir
	runq *File
	ch   chan bool
	done bool
	srv  *npsrv.NpServer
	*fslib.FsLib
}

func MakeSchedd() *Sched {
	sd := &Sched{}
	sd.cond = sync.NewCond(&sd.mu)
	sd.load = 0
	sd.nid = 1 // 1 is runq
	sd.ls = make(map[string]*Lambda)
	sd.root = sd.makeDir([]string{}, np.DMDIR, nil)
	sd.runq = sd.makeFile([]string{"runq"}, 0, sd.root)
	sd.ch = make(chan bool)
	sd.root.time = time.Now().Unix()
	db.Name("schedd")

	ip, err := fsclnt.LocalIP()
	if err != nil {
		log.Fatalf("LocalIP %v %v\n", fslib.SCHED, err)
	}
	sd.srv = npsrv.MakeNpServer(sd, ip+":0")
	fsl := fslib.MakeFsLib("schedd")
	sd.FsLib = fsl
	err = fsl.PostService(sd.srv.MyAddr(), fslib.SCHED)
	if err != nil {
		log.Fatalf("PostService failed %v %v\n", fslib.SCHED, err)
	}
	return sd
}

func (sd *Sched) Connect(conn net.Conn) npsrv.NpAPI {
	return npo.MakeNpConn(sd, conn)
}

func (sd *Sched) WatchTable() *npo.WatchTable {
	return nil
}

func (sd *Sched) ConnTable() *npo.ConnTable {
	return nil
}

func (sd *Sched) Done() {
	sd.mu.Lock()
	sd.done = true
	sd.mu.Unlock()
	sd.ch <- true
	sd.cond.Broadcast()
}

func (sd *Sched) Work() {
	<-sd.ch
}

func (sd *Sched) RootAttach(uname string) (npo.NpObj, npo.CtxI) {
	return sd.root, nil
}

func (sd *Sched) String() string {
	s := ""
	for _, l := range sd.ls {
		l.mu.Lock()
		defer l.mu.Unlock()
		s += l.String()
		s += "\n"
	}
	return s
}

func (sd *Sched) uid() uint64 {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	sd.nid += 1
	return sd.nid
}

func (sd *Sched) spawn(l *Lambda) {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	sd.ls[l.Pid] = l
	sd.cond.Signal()
}

// Lock & find lambda
func (sd *Sched) findLambda(pid string) *Lambda {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	return sd.findLambdaS(pid)
}

func (sd *Sched) findLambdaS(pid string) *Lambda {
	l, _ := sd.ls[pid]
	return l
}

func (sd *Sched) delLambda(pid string) {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	delete(sd.ls, pid)
}

func (sd *Sched) decLoad() {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	sd.load -= 1
	sd.cond.Signal()
}

// wakeup lambdas that have pid as an exit dependency
func (sd *Sched) wakeupExit(pid string) {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	for _, l := range sd.ls {
		l.startExitDep(pid)
	}
	sd.cond.Signal()
}

// Caller holds sd lock
func (sd *Sched) findRunnable() *Lambda {
	for _, l := range sd.ls {
		if l.isRunnable() {
			return l
		}
	}
	return nil
}

// Caller holds sd lock
func (sd *Sched) findRunnableWaitingConsumer() *Lambda {
	for _, l := range sd.ls {
		if l.runnableWaitingConsumer() {
			return l
		}
	}
	return nil
}

// Select a random locald instance to run on
func (sd *Sched) selectLocaldIp() (string, error) {
	ips, err := sd.ReadDir(fslib.LOCALD_ROOT)
	if err != nil {
		log.Printf("Schedd error reading localds dir\n: %v", err)
		return "", err
	}
	n := rand.Int() % len(ips)
	return ips[n].Name, nil
}

func (sd *Sched) findRunnableLambda() ([]byte, error) {
	db.DLPrintf("SCHEDD", "findRunnableLambda called\n")
	sd.mu.Lock()
	defer sd.mu.Unlock()
	for !sd.done {
		db.DLPrintf("SCHEDD", "findRunnableLambda looking for one %v\n", sd)
		l := sd.findRunnableWaitingConsumer()
		if l != nil {
			l.changeStatus("Started")
			if l.Program == NO_OP_LAMBDA {
				go l.writeExitStatus("OK")
				continue
			} else {
				db.DLPrintf("SCHEDD", "findRunnableLambda marshalling %v\n", l.Attr())
				return json.Marshal(*l.Attr())
			}
		} else {
			l = sd.findRunnable()
			if l != nil {
				l.changeStatus("Started")
				if l.Program == NO_OP_LAMBDA {
					go l.writeExitStatus("OK")
					continue
				} else {
					db.DLPrintf("SCHEDD", "findRunnableLambda marshalling %v\n", l.Attr())
					return json.Marshal(*l.Attr())
				}
			}
			db.DLPrintf("SCHEDD", "findRunnableLambda going to sleep %v\n", sd)
			sd.cond.Wait()
		}
	}
	return []byte{}, io.EOF
}
