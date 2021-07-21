package locald

import (
	//	"github.com/sasha-s/go-deadlock"
	"encoding/json"
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	db "ulambda/debug"
	"ulambda/fslib"
	"ulambda/linuxsched"
	np "ulambda/ninep"
)

type Lambda struct {
	//	mu deadlock.Mutex
	mu      sync.Mutex
	Program string
	Pid     string
	Args    []string
	Env     []string
	Dir     string
	Stdout  string
	Stderr  string
	SysPid  int
	attr    *fslib.Attr
	ld      *LocalD
	// XXX add fields (e.g. CPU mask, etc.)
}

// XXX init/run pattern seems a bit redundant...
func (l *Lambda) init(a []byte) error {
	var attr fslib.Attr
	err := json.Unmarshal(a, &attr)
	if err != nil {
		log.Printf("Locald unmarshalling error: %v, %v", err, a)
		return err
	}
	l.Program = attr.Program
	l.Pid = attr.Pid
	l.Args = attr.Args
	l.Env = attr.Env
	l.Dir = attr.Dir
	l.Stdout = "" // XXX: add to or infer from attr
	l.Stderr = "" // XXX: add to or infer from attr
	l.attr = &attr
	db.DLPrintf("LOCALD", "Locald init: %v\n", attr)
	d1 := l.ld.makeDir([]string{attr.Pid}, np.DMDIR, l.ld.root)
	d1.time = time.Now().Unix()
	return nil
}

func (l *Lambda) wait(cmd *exec.Cmd) {
	err := cmd.Wait()
	if err != nil {
		log.Printf("Lambda %v finished with error: %v", l.attr, err)
		l.ld.Exiting(l.attr.Pid, err.Error())
		return
	}

	// Notify schedd that the process exited
	l.ld.Exiting(l.attr.Pid, "OK")
}

func (l *Lambda) run(cores []uint) error {
	db.DLPrintf("LOCALD", "Locald run: %v\n", l.attr)

	// Don't run anything if this is a no-op
	if l.Program == NO_OP_LAMBDA {
		// XXX Should perhaps do this asynchronously, but worried about fsclnt races
		l.ld.Exiting(l.Pid, "OK")
		return nil
	}

	// XXX Hack to get perf stat to work cleanly... we probably want to do proper
	// stdout/stderr redirection eventually...
	var args []string
	var stdout io.Writer
	var stderr io.Writer
	if l.Program == "bin/perf" {
		args = l.Args
		fname := "/tmp/perf-stat-" + l.Pid + ".out"
		file, err := os.Create(fname)
		if err != nil {
			log.Fatalf("Error creating perf stat output file: %v, %v", fname, err)
		}
		stdout = file
		stderr = file
	} else {
		args = append([]string{l.Pid}, l.Args...)
		stdout = os.Stdout
		stderr = os.Stderr
	}

	env := append(os.Environ(), l.Env...)
	cmd := exec.Command(l.ld.bin+"/"+l.Program, args...)
	cmd.Env = env
	cmd.Dir = l.Dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Start()
	if err != nil {
		log.Printf("Locald run error: %v, %v\n", l.attr, err)
		return err
	}

	l.SysPid = cmd.Process.Pid
	// XXX May want to start the process with a certain affinity (using taskset)
	// instead of setting the affinity after it starts
	l.setCpuAffinity(cores)

	l.wait(cmd)
	db.DLPrintf("LOCALD", "Locald ran: %v\n", l.attr)

	return nil
}

func (l *Lambda) setCpuAffinity(cores []uint) {
	m := &linuxsched.CPUMask{}
	for _, i := range cores {
		m.Set(i)
	}
	err := linuxsched.SchedSetAffinityAllTasks(l.SysPid, m)
	if err != nil {
		log.Printf("Error setting CPU affinity for child lambda: %v", err)
	}
}

func (l *Lambda) getConsumers() []string {
	consumers := []string{}
	for _, pair := range l.attr.PairDep {
		if pair.Producer != l.Pid {
			log.Fatalf("Trying to get consumers from lambda, but isn't producer: %v", l)
		}
		consumers = append(consumers, pair.Consumer)
	}
	return consumers
}
