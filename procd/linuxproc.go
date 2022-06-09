package procd

import (
	"log"
	"os"
	"os/exec"
	"path"
	"sync"

	//	"github.com/sasha-s/go-deadlock"

	db "ulambda/debug"
	"ulambda/fs"
	"ulambda/linuxsched"
	"ulambda/namespace"
	np "ulambda/ninep"
	"ulambda/proc"
	"ulambda/rand"
)

type Tstatus uint8

const (
	PROC_RUNNING Tstatus = iota
	PROC_QUEUED
)

type LinuxProc struct {
	//	mu deadlock.Mutex
	fs.Inode
	mu      sync.Mutex
	SysPid  int
	NewRoot string
	Env     []string
	attr    *proc.Proc
	pd      *Procd
}

func makeProc(pd *Procd, a *proc.Proc) *LinuxProc {
	p := &LinuxProc{}
	p.pd = pd
	p.attr = a
	p.NewRoot = path.Join(namespace.NAMESPACE_DIR, p.attr.Pid.String()+rand.String(16))
	db.DPrintf("PROCD", "Procd init: %v\n", p)
	p.Env = append(os.Environ(), a.GetEnv(p.pd.addr, p.NewRoot)...)
	return p
}

func (p *LinuxProc) wait(cmd *exec.Cmd) {
	defer p.pd.fs.finish(p)
	err := cmd.Wait()
	if err != nil {
		db.DPrintf("PROCD_ERR", "Proc %v finished with error: %v\n", p.attr, err)
		p.pd.procclnt.ExitedProcd(p.attr.Pid, p.attr.ProcDir, p.attr.ParentDir, proc.MakeStatusErr(err.Error(), nil))
		return
	}

	err = namespace.Destroy(p.NewRoot)
	if err != nil {
		db.DPrintf("PROCD_ERR", "Error namespace destroy: %v", err)
	}
}

func (p *LinuxProc) run(cores []uint) error {
	db.DPrintf("PROCD", "Procd run: %v\n", p.attr)

	// Make the proc's procdir
	if err := p.pd.procclnt.MakeProcDir(p.attr.Pid, p.attr.ProcDir, p.attr.IsPrivilegedProc()); err != nil {
		db.DPrintf("PROCD_ERR", "Err procd MakeProcDir: %v\n", err)
	}

	cmd := exec.Command(path.Join(np.UXROOT, p.pd.realmbin, p.attr.Program), p.attr.Args...)
	cmd.Env = p.Env
	cmd.Dir = p.attr.Dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	namespace.SetupProc(cmd)
	err := cmd.Start()
	if err != nil {
		db.DPrintf("PROCD_ERR", "Procd run error: %v, %v\n", p.attr, err)
		p.pd.procclnt.ExitedProcd(p.attr.Pid, p.attr.ProcDir, p.attr.ParentDir, proc.MakeStatusErr(err.Error(), nil))
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

func (p *LinuxProc) setCpuAffinity(cores []uint) {
	m := &linuxsched.CPUMask{}
	for _, i := range cores {
		m.Set(i)
	}
	err := linuxsched.SchedSetAffinityAllTasks(p.SysPid, m)
	if err != nil {
		log.Printf("Error setting CPU affinity for child lambda: %v", err)
	}
}
