package schedd

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	// "time"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
)

// XXX monitor, boost

const (
	MAXLOAD = 8 // XXX bogus, controls parallelism
)

type SchedDev struct {
	sd *Sched
}

func (sdev *SchedDev) Write(off np.Toffset, data []byte) (np.Tsize, error) {
	var err error
	t := string(data)
	log.Printf("SchedDev.write %v\n", t)
	if strings.HasPrefix(t, "Spawn") {
		err = sdev.sd.spawn(t[len("Spawn "):])
	} else if strings.HasPrefix(t, "Started") {
		sdev.sd.started(t[len("Started "):])
	} else if strings.HasPrefix(t, "Exiting") {
		sdev.sd.exiting(strings.TrimSpace(t[len("Exiting "):]))
	} else if strings.HasPrefix(t, "Exit") { // must go after Exiting
		sdev.sd.exit()
	} else {
		return 0, fmt.Errorf("Write: unknown command %v\n", t)
	}
	return np.Tsize(len(data)), err
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
	mu   sync.Mutex
	cond *sync.Cond
	*fslib.FsLibSrv
	load int // XXX bogus
	ls   map[string]*Lambda
	done bool
}

func MakeSchedd() *Sched {
	sd := &Sched{}
	sd.cond = sync.NewCond(&sd.mu)
	fsl, err := fslib.InitFs(fslib.SCHED, &SchedDev{sd})
	if err != nil {
		log.Fatalf("InitFs: %v\n", err)

	}
	sd.FsLibSrv = fsl
	sd.load = 0
	sd.ls = make(map[string]*Lambda)
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

func (sd *Sched) ps() string {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	s := fmt.Sprintf("Load %v\n", sd.load)
	s += sd.String()
	return s
}

func (sd *Sched) exit() {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	sd.done = true
	sd.cond.Signal()
}

func (sd *Sched) spawn(ls string) error {
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
		return err
	}
	l.pid = attr.Pid
	l.program = attr.Program
	l.args = attr.Args
	l.env = attr.Env
	for _, p := range attr.PairDep {
		if l.pid != p.Producer {
			c, ok := sd.ls[p.Producer]
			if ok {
				l.prodDep[p.Producer] = c.isRunnning()
			} else {
				l.prodDep[p.Producer] = false
			}
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
		return fmt.Errorf("Spawn %v already exists\n", l.pid)

	}

	l.setStatus()
	db.DPrintf("Spawn %v\n", l)
	sd.cond.Signal()
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
	l.changeStatus("Running")
	l.startConsDep()
	sd.runScheduler()
}

func (sd *Sched) exiting(pid string) {
	l := sd.findLambda(pid)
	if l != nil {
		sd.decLoad()
		l.stopProducers()
		l.waitExit()
		sd.wakeupExit(pid)
		sd.delLambda(pid)
	}
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
		// time.Sleep(time.Duration(1) * time.Millisecond)
	}
}

// timeout := int64(st.Mtime) + 5
// if timeout < time.Now().Unix() {
