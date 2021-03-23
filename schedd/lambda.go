package schedd

import (
	"encoding/json"
	"fmt"
	// "github.com/sasha-s/go-deadlock"
	"log"
	//	"os"
	//	"os/exec"
	"strings"
	"sync"

	db "ulambda/debug"
	"ulambda/fslib"
)

type Lambda struct {
	//	mu deadlock.Mutex
	mu             sync.Mutex
	cond           *sync.Cond
	condWait       *sync.Cond
	sd             *Sched
	Pid            string
	uid            uint64
	LocaldInstance string
	Status         string
	ExitStatus     string
	Program        string
	Dir            string
	Args           []string
	Env            []string
	ConsDep        map[string]bool // if true, consumer has finished
	ProdDep        map[string]bool // if true, producer is running
	ExitDep        map[string]bool
	obj            *Obj
	attr           *fslib.Attr
}

func makeLambda(sd *Sched, a string, o *Obj) *Lambda {
	l := &Lambda{}
	l.sd = sd
	l.cond = sync.NewCond(&l.mu)
	l.condWait = sync.NewCond(&l.mu)
	l.ConsDep = make(map[string]bool)
	l.ProdDep = make(map[string]bool)
	l.ExitDep = make(map[string]bool)
	l.Pid = a
	l.uid = sd.uid()
	l.Status = "Init"
	l.obj = o
	return l
}

func (l *Lambda) pairDep(pairdep []fslib.PDep) {
	for _, p := range pairdep {
		if l.Pid != p.Producer {
			c, ok := l.sd.ls[p.Producer]
			if ok {
				l.ProdDep[p.Producer] = c.isRunnningL()
			} else {
				l.ProdDep[p.Producer] = false
			}
		}
		if l.Pid != p.Consumer {
			l.ConsDep[p.Consumer] = false
		}
	}
}

func (l *Lambda) init(a []byte) error {
	l.sd.mu.Lock()
	defer l.sd.mu.Unlock()
	l.mu.Lock()
	defer l.mu.Unlock()

	var attr fslib.Attr

	err := json.Unmarshal(a, &attr)
	if err != nil {
		return err
	}
	l.attr = &attr
	l.Pid = attr.Pid
	l.LocaldInstance = LOCALD_UNASSIGNED
	l.Program = attr.Program
	l.Args = attr.Args
	l.Env = attr.Env
	l.Dir = attr.Dir
	l.pairDep(attr.PairDep)
	for _, p := range attr.ExitDep {
		l.ExitDep[p] = false
	}
	l.pruneExitDepLS()
	if l.runnableConsumerL() {
		l.Status = "Runnable"
	} else {
		l.Status = "Waiting"
	}
	return nil
}

func (l *Lambda) String() string {
	str := fmt.Sprintf("λ pid %v st %v %v args %v env %v dir %v cons %v prod %v exit %v",
		l.Pid, l.Status, l.Program, l.Args, l.Env, l.Dir, l.ConsDep, l.ProdDep,
		l.ExitDep)
	return str
}

func (l *Lambda) Attr() *fslib.Attr {
	return l.attr
}

func (l *Lambda) writeExitStatus(status string) {
	// Always take the sd lock before the l lock
	l.sd.mu.Lock()
	l.mu.Lock()

	l.ExitStatus = status
	db.DLPrintf("SCHEDD", "Exit %v status %v; Wakeup waiters for %v\n", l.Pid, status, l)
	l.condWait.Broadcast()

	l.mu.Unlock()

	l.stopProducersS()

	l.sd.mu.Unlock()

	l.waitExit()

	l.sd.delLambda(l.Pid)
	l.sd.decLoad()
	l.sd.wakeupExit(l.Pid)
}

func (l *Lambda) writeStatus(status string) error {
	l.mu.Lock()
	if l.Status != "Started" {
		l.mu.Unlock()
		return fmt.Errorf("Cannot write to status %v", l.Status)
	}
	l.Status = "Running"
	db.DLPrintf("SCHEDD", "Running %v\n", l.Pid)
	l.mu.Unlock()

	l.startConsDep()
	l.sd.cond.Signal()
	return nil
}

func (l *Lambda) changeStatus(new string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.Status = new
	return nil
}

func (l *Lambda) swapExitDependency(swap string) {
	l.sd.mu.Lock()
	defer l.sd.mu.Unlock()
	l.mu.Lock()
	defer l.mu.Unlock()
	db.DLPrintf("SCHEDD", "Swapping exit dep %v for lambda %v\n", swap, l.Pid)

	s := strings.Split(strings.TrimSpace(swap), " ")
	from := s[0]
	to := s[1]

	// Check that the lambda we're swapping to still exists, else ignore
	if _, ok := l.sd.ls[to]; ok {
		// Check if present & false (hasn't exited yet)
		if val, ok := l.ExitDep[from]; ok && !val {
			db.DLPrintf("SCHEDD", "Swapping exit dep %v for lambda %v\n", swap, l.Pid)
			l.ExitDep[to] = false
			l.ExitDep[from] = true
		}
	} else {
		db.DLPrintf("SCHEDD", "Tried to swap exit dep %v for lambda %v, but it didn't exist\n", swap, l.Pid)
	}
}

func (l *Lambda) run() error {
	db.DLPrintf("SCHEDD", "Run %v\n", l)
	err := l.changeStatus("Started")
	if err != nil {
		return err
	}

	// If this was a no-op, exit immediately
	if l.Program == NO_OP_LAMBDA {
		go l.writeExitStatus("OK")
		return nil
	}

	ip, err := l.sd.selectLocaldIp()
	if err != nil {
		log.Printf("Schedd failed to select local ip to run lambda\n")
	}
	l.LocaldInstance = ip
	err = l.sd.RunLocal(ip, l.attr)
	if err != nil {
		log.Printf("Schedd failed to run local lambda: %v, %v\n", l, err)
	}
	return nil
}

func (l *Lambda) startConsDep() {
	l.sd.mu.Lock()
	defer l.sd.mu.Unlock()
	l.mu.Lock()
	defer l.mu.Unlock()

	for p, _ := range l.ConsDep {
		c := l.sd.findLambdaS(p)
		if c != nil && l.Pid != c.Pid {
			c.markProducer(l.Pid)
		}
	}
}

func (l *Lambda) startExitDep(pid string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	_, ok := l.ExitDep[pid]
	if ok {
		l.ExitDep[pid] = true
		for _, b := range l.ExitDep {
			if !b {
				return
			}
		}
		l.Status = "Runnable"
	}
}

// XXX This function isn't entirely atomic. I don't think this is an issue for
// now, but it seems like a reasonably performant way of avoiding a deadlock.
func (l *Lambda) stopProducersS() {
	// XXX a hack to avoid holding locks & deadlocking...
	l.mu.Lock()
	pDep := map[string]bool{}
	for k, v := range l.ProdDep {
		pDep[k] = v
	}
	l.mu.Unlock()

	for p, _ := range pDep {
		c := l.sd.findLambdaS(p)
		if c != nil && l.Pid != c.Pid {
			c.markConsumerS(l.Pid)
		}
	}
}

func (l *Lambda) pruneExitDepLS() {
	for dep, done := range l.ExitDep {
		// If schedd knows nothing about the exit dep, ignore it
		if _, ok := l.sd.ls[dep]; !ok && !done {
			delete(l.ExitDep, dep)
		}
	}
}

func (l *Lambda) markProducer(pid string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ProdDep[pid] = true
}

func (l *Lambda) markExit(pid string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ExitDep[pid] = true
}

func (l *Lambda) markConsumerS(pid string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ConsDep[pid] = true
	l.cond.Signal()
}

func (l *Lambda) runnableConsumerL() bool {
	if len(l.ExitDep) != 0 {
		return false
	}
	run := true
	for _, b := range l.ProdDep {
		if !b {
			return false
		}
	}
	return run
}

func (l *Lambda) runnableWaitingConsumer() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.Status != "Waiting" {
		return false
	}
	return l.runnableConsumerL()
}

// Wait for consumers that depend on me to exit too.
func (l *Lambda) waitExit() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for !l.exitableL() {
		l.cond.Wait()
	}
}

func (l *Lambda) exitableL() bool {
	exit := true
	for _, b := range l.ConsDep {
		if !b {
			return false
		}
	}
	return exit
}

func (l *Lambda) isRunnable() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.Status == "Runnable"
}

func (l *Lambda) isRunnningL() bool {
	return l.Status == "Running"
}

// A caller wants to Wait for l to exit.
func (l *Lambda) waitForL() string {
	db.DLPrintf("SCHEDD", "Wait for %v\n", l)
	if l.Status != "Exiting" {
		l.condWait.Wait()
	}
	return l.ExitStatus
}
