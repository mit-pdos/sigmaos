package schedd

import (
	"encoding/json"
	"fmt"
	//	"github.com/sasha-s/go-deadlock"
	"log"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
)

type Lambda struct {
	//	mu         deadlock.Mutex
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
	Dir        string
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
	l.Status = "Init"
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
	l.Pid = attr.Pid
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

func (l *Lambda) continueing(a []byte) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	var attr fslib.Attr

	err := json.Unmarshal(a, &attr)
	if err != nil {
		return err
	}
	if l.Pid != attr.Pid {
		return fmt.Errorf("continueLambda: pids don't match %v\n", attr.Pid)
	}
	l.Program = attr.Program
	l.Args = attr.Args
	l.Env = attr.Env
	l.Dir = attr.Dir
	l.pairDep(attr.PairDep)
	for _, p := range attr.ExitDep {
		l.ExitDep[p] = false
	}

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

func (l *Lambda) qid(isL bool) np.Tqid {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.qidL(isL)
}

func (l *Lambda) qidL(isL bool) np.Tqid {
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
		st.Qid = l.qidL(true)
		st.Name = l.Pid
	} else {
		st.Mode = np.Tperm(0777) | FIELD.mode()
		st.Qid = l.qidL(false)
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
		l.waitForL()
		b = []byte(l.ExitStatus)
	case "Status":
		b = []byte(l.Status)
	case "Program":
		b = []byte(l.Program)
	case "Pid":
		b = []byte(l.Pid)
	case "Dir":
		b = []byte(l.Dir)
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

// XXX might want to clean up the locking here
func (l *Lambda) writeExitStatus(status string) {
	// Always take the sd lock before the l lock
	l.sd.mu.Lock()
	l.mu.Lock()

	l.ExitStatus = status
	db.DPrintf("Exit %v status %v; Wakeup waiters for %v\n", l.Pid, status, l)
	l.condWait.Broadcast()
	l.stopProducersLS()

	l.sd.mu.Unlock()

	l.waitExitL()

	l.mu.Unlock()

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
	case "ExitDep":
		l.swapExitDependency(string(data))
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

func (l *Lambda) swapExitDependency(swap string) {
	l.sd.mu.Lock()
	defer l.sd.mu.Unlock()
	l.mu.Lock()
	defer l.mu.Unlock()
	db.DPrintf("Swapping exit dep %v for lambda %v\n", swap, l.Pid)

	s := strings.Split(strings.TrimSpace(swap), " ")
	from := s[0]
	to := s[1]

	// Check that the lambda we're swapping to still exists, else ignore
	if _, ok := l.sd.ls[to]; ok {
		// Check if present & false (hasn't exited yet)
		if val, ok := l.ExitDep[from]; ok && !val {
			db.DPrintf("Swapping exit dep %v for lambda %v\n", swap, l.Pid)
			l.ExitDep[to] = false
			l.ExitDep[from] = true
		}
	} else {
		db.DPrintf("Tried to swap exit dep %v for lambda %v, but it didn't exist\n", swap, l.Pid)
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

// XXX Should we lock l's fields here?
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
	cmd.Dir = l.Dir
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

func (l *Lambda) stopProducersLS() {
	for p, _ := range l.ProdDep {
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
func (l *Lambda) waitExitL() {
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
	db.DPrintf("Wait for %v\n", l)
	if l.Status != "Exiting" {
		l.condWait.Wait()
	}
	return l.ExitStatus
}
