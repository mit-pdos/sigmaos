package procd

import (
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"sync"

	//	"github.com/sasha-s/go-deadlock"

	db "ulambda/debug"
	"ulambda/fs"
	"ulambda/linuxsched"
	"ulambda/machine"
	"ulambda/namespace"
	"ulambda/proc"
	"ulambda/rand"
)

type Tstatus uint8

const (
	PROC_RUNNING Tstatus = iota
	PROC_QUEUED
)

type Proc struct {
	//	mu deadlock.Mutex
	fs.Inode
	mu      sync.Mutex
	Program string
	Pid     proc.Tpid
	Args    []string
	Env     []string
	Dir     string
	Stdout  string
	Stderr  string
	SysPid  int
	NewRoot string
	attr    *proc.Proc
	pd      *Procd
	// XXX add fields (e.g. CPU mask, etc.)
}

// XXX init/run pattern seems a bit redundant...
func (p *Proc) init(a *proc.Proc) {
	p.attr = a
	p.Program = a.Program
	p.Pid = a.Pid
	p.Args = a.Args
	p.Dir = a.Dir
	p.NewRoot = path.Join(namespace.NAMESPACE_DIR, p.Pid.String()+rand.String(16))
	db.DPrintf("PROCD", "Procd init: %v\n", p)
	p.Env = append(os.Environ(), a.GetEnv(p.pd.addr, p.NewRoot)...)
	p.Stdout = "" // XXX: add to or infer from p
	p.Stderr = "" // XXX: add to or infer from p
}

func (p *Proc) wait(cmd *exec.Cmd) {
	defer p.pd.fs.finish(p)
	err := cmd.Wait()
	if err != nil {
		db.DPrintf("PROCD_ERR", "Proc %v finished with error: %v\n", p.attr, err)
		p.pd.procclnt.ExitedProcd(p.Pid, p.attr.ProcDir, p.attr.ParentDir, proc.MakeStatusErr(err.Error(), nil))
		return
	}

	err = namespace.Destroy(p.NewRoot)
	if err != nil {
		db.DPrintf("PROCD_ERR", "Error namespace destroy: %v", err)
	}
}

func (p *Proc) run(cores []uint) error {
	db.DPrintf("PROCD", "Procd run: %v\n", p.attr)

	// XXX Hack to get perf stat to work cleanly... we probably want to do proper
	// stdout/stderr redirection eventually...
	var args []string
	var stdout io.Writer
	var stderr io.Writer
	args = p.Args
	stdout = os.Stdout
	stderr = os.Stderr

	// Make the proc's procdir
	if err := p.pd.procclnt.MakeProcDir(p.Pid, p.attr.ProcDir, p.attr.IsPrivilegedProc()); err != nil {
		db.DPrintf("PROCD_ERR", "Err procd MakeProcDir: %v\n", err)
	}

	cmd := exec.Command(path.Join(machine.BIN, p.Program), args...)
	cmd.Env = p.Env
	cmd.Dir = p.Dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	namespace.SetupProc(cmd)
	err := cmd.Start()
	if err != nil {
		db.DPrintf("PROCD_ERR", "Procd run error: %v, %v\n", p.attr, err)
		p.pd.procclnt.ExitedProcd(p.Pid, p.attr.ProcDir, p.attr.ParentDir, proc.MakeStatusErr(err.Error(), nil))
		return err
	}

	p.SysPid = cmd.Process.Pid
	// XXX May want to start the process with a certain affinity (using taskset)
	// instead of setting the affinity after it starts
	p.setCpuAffinity(cores)

	p.wait(cmd)
	db.DPrintf("PROCD", "Procd ran: %v\n", p.attr)

	return nil
}

func (p *Proc) setCpuAffinity(cores []uint) {
	m := &linuxsched.CPUMask{}
	for _, i := range cores {
		m.Set(i)
	}
	err := linuxsched.SchedSetAffinityAllTasks(p.SysPid, m)
	if err != nil {
		log.Printf("Error setting CPU affinity for child lambda: %v", err)
	}
}
