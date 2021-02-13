package schedd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"reflect"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
)

type Lambda struct {
	mu         sync.Mutex
	cond       *sync.Cond
	condWait   *sync.Cond
	sd         *Sched
	Pid        string
	uid        uint64
	time       int64
	Status     string
	ExitStatus string
	Program    string
	Args       []string
	Env        []string
	ConsDep    map[string]bool // if true, consumer has finished
	ProdDep    map[string]bool // if true, producer is running
	ExitDep    map[string]bool
}

func makeLambda(sd *Sched, a string) *Lambda {
	l := &Lambda{}
	l.sd = sd
	l.cond = sync.NewCond(&l.mu)
	l.condWait = sync.NewCond(&l.mu)
	l.ConsDep = make(map[string]bool)
	l.ProdDep = make(map[string]bool)
	l.ExitDep = make(map[string]bool)
	l.Pid = a
	l.time = time.Now().Unix()
	l.uid = sd.uid()
	l.Status = "init"
	return l
}

func (l *Lambda) initLambda(a []byte) error {
	var attr fslib.Attr

	err := json.Unmarshal(a, &attr)
	if err != nil {
		return err
	}
	l.Pid = attr.Pid
	l.Program = attr.Program
	l.Args = attr.Args
	l.Env = attr.Env
	for _, p := range attr.PairDep {
		if l.Pid != p.Producer {
			c, ok := l.sd.ls[p.Producer]
			if ok {
				l.ProdDep[p.Producer] = c.isRunnning()
			} else {
				l.ProdDep[p.Producer] = false
			}
		}
		if l.Pid != p.Consumer {
			l.ConsDep[p.Consumer] = false
		}
	}
	for _, p := range attr.ExitDep {
		l.ExitDep[p] = false
	}
	if l.runnableConsumer() {
		l.Status = "Runnable"
	} else {
		l.Status = "Waiting"
	}
	return nil
}

func (l *Lambda) String() string {
	str := fmt.Sprintf("λ pid %v st %v %v args %v env %v cons %v prod %v exit %v",
		l.Pid, l.Status, l.Program, l.Args, l.Env, l.ConsDep, l.ProdDep,
		l.ExitDep)
	return str
}

func (l *Lambda) qid(isL bool) np.Tqid {
	if isL {
		return np.MakeQid(np.Qtype(np.DMDIR>>np.QTYPESHIFT),
			np.TQversion(0), np.Tpath(l.uid))
	} else {
		return np.MakeQid(np.Qtype(np.DMDEVICE>>np.QTYPESHIFT),
			np.TQversion(0), np.Tpath(l.uid))
	}
}

func (l *Lambda) stat(name string) *np.Stat {
	l.mu.Lock()
	defer l.mu.Unlock()

	st := &np.Stat{}
	if name == "lambda" {
		st.Mode = np.Tperm(0777) | LAMBDA.mode()
		st.Qid = l.qid(true)
		st.Name = l.Pid
	} else {
		st.Mode = np.Tperm(0777) | FIELD.mode()
		st.Qid = l.qid(false)
		st.Name = name
	}
	st.Mtime = uint32(l.time)
	return st
}

func (l *Lambda) ls() []*np.Stat {
	st := []*np.Stat{}

	v := reflect.ValueOf(Lambda{})
	for i := 0; i < v.NumField(); i++ {
		n := v.Type().Field(i).Name
		r, _ := utf8.DecodeRuneInString(n)
		if unicode.IsUpper(r) {
			st = append(st, l.stat(n))
		}
	}
	return st
}

// XXX reflection also requires a switch but just on kind, perhaps better
func (l *Lambda) readField(f string) ([]byte, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	var b []byte
	switch f {
	case "ExitStatus":
		l.waitFor()
		b = []byte(l.ExitStatus)
	case "Status":
		b = []byte(l.Status)
	case "Program":
		b = []byte(l.Program)
	case "Pid":
		b = []byte(l.Pid)
	case "Args":
		return json.Marshal(l.Args)
	case "Env":
		return json.Marshal(l.Env)
	case "ConsDep":
		return json.Marshal(l.ConsDep)
	case "ProdDep":
		return json.Marshal(l.ProdDep)
	case "ExitDep":
		return json.Marshal(l.ExitDep)
	default:
		return nil, fmt.Errorf("Unreadable field %v", f)
	}
	return b, nil
}

func (l *Lambda) writeExitStatus(status string) {
	l.mu.Lock()

	l.ExitStatus = status
	db.DPrintf("Exit %v; Wakeup waiters for %v\n", l.Pid, l)
	l.condWait.Broadcast()
	l.stopProducers()
	l.waitExit()

	l.mu.Unlock()

	l.sd.wakeupExit(l.Pid)
	l.sd.delLambda(l.Pid)
}

func (l *Lambda) writeStatus(status string) error {
	l.mu.Lock()
	if l.Status != "Started" {
		return fmt.Errorf("Cannot write to status %v", l.Status)
	}
	l.Status = "Running"
	db.DPrintf("Running %v\n", l.Pid)
	l.mu.Unlock()

	l.startConsDep()
	l.sd.cond.Signal()
	return nil
}

func (l *Lambda) writeField(f string, data []byte) error {
	switch f {
	case "ExitStatus":
		l.writeExitStatus(string(data))
	case "Status":
		l.writeStatus(string(data))
	default:
		return fmt.Errorf("Unwritable field %v", f)
	}
	return nil
}

func (l *Lambda) changeStatus(new string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.Status = new
	return nil
}

// XXX Might want to optimize this.
func (l *Lambda) swapExitDependency(depSwaps map[string]string) {
	// Assuming len(depSwaps) << len(l.exitDeps)
	for from, to := range depSwaps {
		// Check if present & false (hasn't exited yet)
		if val, ok := l.ExitDep[from]; ok && !val {
			l.ExitDep[to] = false
			l.ExitDep[from] = true
		}
	}
}

// XXX if remote, keep-alive?
func (l *Lambda) wait(cmd *exec.Cmd) {
	err := cmd.Wait()
	if err != nil {
		l.mu.Lock()
		defer l.mu.Unlock()
		log.Printf("Lambda %v finished with error: %v", l, err)
	}
}

// XXX if had remote machines, this would be run on the remote machine
// maybe we should have machines register with ulambd; have a
// directory with machines?
func (l *Lambda) run() error {
	db.DPrintf("Run %v\n", l)
	err := l.changeStatus("Started")
	if err != nil {
		return err
	}
	args := append([]string{l.Pid}, l.Args...)
	env := append(os.Environ(), l.Env...)
	cmd := exec.Command(l.Program, args...)
	cmd.Env = env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		return err
	}
	go l.wait(cmd)
	return nil
}

func (l *Lambda) startConsDep() {
	for p, _ := range l.ConsDep {
		c := l.sd.findLambda(p)
		if c != nil {
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

func (l *Lambda) stopProducers() {
	for p, _ := range l.ProdDep {
		c := l.sd.findLambda(p)
		if c != nil {
			c.markConsumer(l.Pid)
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

func (l *Lambda) markConsumer(pid string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ConsDep[pid] = true
	l.cond.Signal()
}

// caller holds lock
func (l *Lambda) runnableConsumer() bool {
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
	return l.runnableConsumer()
}

// Wait for consumers that depend on me to exit too.
func (l *Lambda) waitExit() {
	for !l.exitable() {
		l.cond.Wait()
	}
}

// Caller hold locks
func (l *Lambda) exitable() bool {
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

func (l *Lambda) isRunnning() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.Status == "Running"
}

// A caller wants to Wait for l to exit.
// Caller must have l locked
func (l *Lambda) waitFor() string {
	db.DPrintf("Wait for %v\n", l)
	if l.Status != "Exiting" {
		l.condWait.Wait()
	}
	return l.ExitStatus
}
