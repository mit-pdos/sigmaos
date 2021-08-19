package procd

import (
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"time"

	//	"github.com/sasha-s/go-deadlock"

	db "ulambda/debug"
	"ulambda/linuxsched"
	np "ulambda/ninep"
	"ulambda/proc"
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
	attr    *proc.Proc
	pd      *Procd
	// XXX add fields (e.g. CPU mask, etc.)
}

// XXX init/run pattern seems a bit redundant...
func (l *Lambda) init(p *proc.Proc) {
	l.Program = p.Program
	l.Pid = p.Pid
	l.Args = p.Args
	l.Env = p.Env
	l.Dir = p.Dir
	l.Stdout = "" // XXX: add to or infer from p
	l.Stderr = "" // XXX: add to or infer from p
	l.attr = p
	db.DLPrintf("PROCD", "Procd init: %v\n", p)
	d1 := l.pd.makeDir([]string{p.Pid}, np.DMDIR, l.pd.root)
	d1.time = time.Now().Unix()
}

func (l *Lambda) wait(cmd *exec.Cmd) {
	err := cmd.Wait()
	if err != nil {
		log.Printf("Lambda %v finished with error: %v", l.attr, err)
		l.pd.Exited(l.attr.Pid)
		return
	}

	// Notify schedd that the process exited
	//	l.pd.Exited(l.attr.Pid)
}

func (l *Lambda) run(cores []uint) error {
	db.DLPrintf("PROCD", "Procd run: %v\n", l.attr)

	// Don't run anything if this is a no-op
	if l.Program == NO_OP_LAMBDA {
		// XXX Should perhaps do this asynchronously, but worried about fsclnt races
		l.pd.Exited(l.Pid)
		return nil
	}

	// XXX Hack to get perf stat to work cleanly... we probably want to do proper
	// stdout/stderr redirection eventually...
	var args []string
	var stdout io.Writer
	var stderr io.Writer
	if l.Program == "bin/user/perf" {
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
	cmd := exec.Command(l.pd.bin+"/"+l.Program, args...)
	cmd.Env = env
	cmd.Dir = l.Dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Start()
	if err != nil {
		log.Printf("Procd run error: %v, %v\n", l.attr, err)
		return err
	}

	l.SysPid = cmd.Process.Pid
	// XXX May want to start the process with a certain affinity (using taskset)
	// instead of setting the affinity after it starts
	l.setCpuAffinity(cores)

	l.wait(cmd)
	db.DLPrintf("PROCD", "Procd ran: %v\n", l.attr)

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
