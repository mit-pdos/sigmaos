package schedd

import (
	"encoding/json"
	//"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	// "time"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/memfsd"
	np "ulambda/ninep"
	"ulambda/npsrv"
)

// XXX monitor, boost

const (
	MAXLOAD = 1 // XXX bogus, controls parallelism
)

type SchedDev struct {
	sd *Sched
}

func (sdev *SchedDev) Write(off np.Toffset, data []byte) (np.Tsize, error) {
	t := string(data)
	db.DPrintf("SchedDev.write %v\n", t)
	if strings.HasPrefix(t, "Spawn") {
		l := strings.TrimLeft(t, "Spawn ")
		sdev.sd.spawn(l)
	} else if strings.HasPrefix(t, "Started") {
		pid := strings.TrimLeft(t, "Started ")
		sdev.sd.started(pid)
	} else if strings.HasPrefix(t, "Exiting") {
		pid := strings.TrimLeft(t, "Exiting ")
		sdev.sd.exiting(pid)
	} else {
		return 0, fmt.Errorf("Write: unknown command %v\n", t)
	}
	return np.Tsize(len(data)), nil
}

func (sdev *SchedDev) Read(off np.Toffset, n np.Tsize) ([]byte, error) {
	if off == 0 {
		s := sdev.sd.ps()
		return []byte(s), nil
	}
	return nil, nil
}

func (sdev *SchedDev) Len() np.Tlength {
	return np.Tlength(len(sdev.sd.ps()))
}

type Sched struct {
	mu     sync.Mutex
	cond   *sync.Cond
	clnt   *fslib.FsLib
	memfsd *memfsd.Fsd
	srv    *npsrv.NpServer
	load   int // XXX bogus
	ls     map[string]*Lambda
}

func MakeSchedd(debug bool) *Sched {
	sd := &Sched{}
	sd.cond = sync.NewCond(&sd.mu)
	sd.clnt = fslib.MakeFsLib(false)
	sd.memfsd = memfsd.MakeFsd(false)
	sd.srv = npsrv.MakeNpServer(sd.memfsd, ":0", false)
	sd.load = 0
	sd.ls = make(map[string]*Lambda)

	err := sd.clnt.Remove(fslib.SCHED)
	if err != nil {
		db.DPrintf("%v didn't exist", fslib.SCHED)
	}
	name := sd.srv.MyAddr()
	err = sd.clnt.Symlink(name+":pubkey:schedd", fslib.SCHED, 0777)
	if err != nil {
		log.Fatal("Symlink error: ", err)
	}

	fs := sd.memfsd.Root()
	_, err = fs.MkNod(fs.RootInode(), fslib.SDEV, &SchedDev{sd})
	if err != nil {
		log.Fatal("Create error: ", err)
	}
	db.SetDebug(debug)
	return sd
}

func (sd *Sched) String() string {
	s := ""
	for _, l := range sd.ls {
		s += fmt.Sprintf("%v\n", l)

	}
	return s
}

func (sd *Sched) ps() string {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	s := fmt.Sprintf("Load %v\n", sd.load)
	s += sd.String()
	return s
}

func (sd *Sched) spawn(ls string) {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	l := &Lambda{}
	l.sd = sd
	l.cond = sync.NewCond(&l.mu)
	l.consDep = make(map[string]bool)
	l.prodDep = make(map[string]bool)
	l.exitDep = make(map[string]bool)
	var attr fslib.Attr
	err := json.Unmarshal([]byte(ls), &attr)
	if err != nil {
		log.Fatal("Unmarshal error ", err)
	}
	l.pid = attr.Pid
	l.program = attr.Program
	l.args = attr.Args
	for _, p := range attr.PairDep {
		if l.pid != p.Producer {
			l.prodDep[p.Producer] = false
		}
		if l.pid != p.Consumer {
			l.consDep[p.Consumer] = false
		}
	}
	for _, p := range attr.ExitDep {
		l.exitDep[p] = false
	}
	_, ok := sd.ls[l.pid]
	if !ok {
		sd.ls[l.pid] = l
	} else {
		log.Fatalf("Spawn %v already exists\n", l.pid)
	}
	l.setStatus()
	db.DPrintf("Spawn %v\n", l)
	sd.cond.Signal()
}

func (sd *Sched) findLambda(pid string) *Lambda {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	l, _ := sd.ls[pid]
	return l
}

func (sd *Sched) delLambda(pid string) {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	delete(sd.ls, pid)
}

func (sd *Sched) runScheduler() {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	sd.cond.Signal()
}

func (sd *Sched) decLoad() {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	sd.load -= 1
	sd.cond.Signal()
}

func (sd *Sched) started(pid string) {
	l := sd.findLambda(pid)
	l.startConsDep()
	sd.runScheduler()
}

func (sd *Sched) exiting(pid string) {
	l := sd.findLambda(pid)
	sd.decLoad()
	l.stopProducers()
	l.waitExit()
	sd.wakeupExit(pid)
	sd.delLambda(pid)
}

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

func (sd *Sched) Scheduler() {
	sd.mu.Lock()
	for {
		l := sd.findRunnableWaitingConsumer()
		if l != nil {
			// XX don't count a consumer against load
			l.run()
		} else {
			if sd.load < MAXLOAD {
				l = sd.findRunnable()
				if l != nil {
					l.run()
					sd.load += 1
				}
			}
			if l == nil || sd.load >= MAXLOAD {
				sd.cond.Wait()
			}
		}
		// time.Sleep(time.Duration(1) * time.Millisecond)
	}
}

// log.Printf("in progress: %v\n", st.Name)
// timeout := int64(st.Mtime) + 5
// if timeout < time.Now().Unix() {
// 	log.Print("REDO ", st.Name)
// 	err = md.clnt.Rename("name/mr/started/"+st.Name,
// 		"name/mr/todo/"+st.Name)
// }
