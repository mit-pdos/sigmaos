package schedd

import (
	"fmt"
	"log"
	"sync"
	// "time"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	"ulambda/npsrv"
)

// XXX monitor, boost

const (
	MAXLOAD = 100 // XXX bogus, controls parallelism
)

type Sched struct {
	mu   sync.Mutex
	cond *sync.Cond
	load int // XXX bogus
	nid  uint64
	ls   map[string]*Lambda
	done bool
	srv  *npsrv.NpServer
}

func MakeSchedd() *Sched {
	sd := &Sched{}
	sd.cond = sync.NewCond(&sd.mu)
	sd.load = 0
	sd.nid = 1 // 1 reserved for dev
	sd.ls = make(map[string]*Lambda)
	// db.SetDebug(true)
	sd.srv = npsrv.MakeNpServer(sd, ":0")
	fsl := fslib.MakeFsLib("sched")
	err := fsl.PostService(sd.srv.MyAddr(), fslib.SCHED)
	if err != nil {
		log.Fatalf("PostService failed %v %v\n", fslib.SCHED, err)
	}
	return sd
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

func (sd *Sched) ps() []*np.Stat {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	dir := []*np.Stat{}
	for _, l := range sd.ls {
		dir = append(dir, l.stat("lambda"))
	}
	return dir
}

func (sd *Sched) spawn(l *Lambda) {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	l.pruneExitDep()
	sd.ls[l.Pid] = l
	sd.cond.Signal()
}

// Exit the scheduler
func (sd *Sched) exit() {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	sd.done = true
	sd.cond.Signal()
}

func (sd *Sched) swapExitDependencies(pids []string) error {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	if len(pids)%2 != 0 {
		return fmt.Errorf("Swapping an odd number of exit deps [%v]\n", pids)
	}
	depSwaps := make(map[string]string)
	for i := 0; i < len(pids); i += 2 {
		from := pids[i]
		to := pids[i+1]
		depSwaps[from] = to
		db.DPrintf("Swapping exit dep [%v] -> [%v]\n", from, to)
	}
	for _, l := range sd.ls {
		l.swapExitDependency(depSwaps)
	}
	return nil
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

func (sd *Sched) Scheduler() {
	sd.mu.Lock()
	for !sd.done {
		l := sd.findRunnableWaitingConsumer()
		if l != nil {
			// XXX don't count starting a consumer against load
			l.run()
			sd.load += 1
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
	}
}

// timeout := int64(st.Mtime) + 5
// if timeout < time.Now().Unix() {
