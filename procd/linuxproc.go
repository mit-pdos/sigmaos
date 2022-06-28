package procd

import (
	"log"
	"os"
	"os/exec"
	"path"
	"time"

	db "ulambda/debug"
	"ulambda/fs"
	"ulambda/linuxsched"
	"ulambda/namespace"
	np "ulambda/ninep"
	"ulambda/proc"
)

type Tstatus uint8

const (
	PROC_RUNNING Tstatus = iota
	PROC_QUEUED
)

type LinuxProc struct {
	fs.Inode
	SysPid       int
	Env          []string
	coresAlloced proc.Tcore
	attr         *proc.Proc
	pd           *Procd
	UtilInfo     struct {
		utime0 uint64
		stime0 uint64
		t0     time.Time
	}
}

func makeLinuxProc(pd *Procd, a *proc.Proc) *LinuxProc {
	p := &LinuxProc{}
	p.pd = pd
	p.attr = a
	db.DPrintf("PROCD", "Procd init: %v\n", p)
	// Finalize the proc env with values related to this physical machine.
	p.attr.FinalizeEnv(p.pd.addr)
	p.Env = append(os.Environ(), p.attr.GetEnv()...)
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

	err = namespace.Destroy(p.attr.LinuxRoot)
	if err != nil {
		db.DPrintf("PROCD_ERR", "Error namespace destroy: %v", err)
	}
}

func (p *LinuxProc) run() error {
	db.DPrintf("PROCD", "Procd run: %v\n", p.attr)

	// Make the proc's procdir
	if err := p.pd.procclnt.MakeProcDir(p.attr.Pid, p.attr.ProcDir, p.attr.IsPrivilegedProc()); err != nil {
		db.DPrintf("PROCD_ERR", "Err procd MakeProcDir: %v\n", err)
	}

	cmd := exec.Command(path.Join(np.UXROOT, p.pd.realmbin, p.attr.Program), p.attr.Args...)
	cmd.Env = p.Env
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
	p.setCpuAffinity()

	p.wait(cmd)
	db.DPrintf("PROCD", "Procd ran: %v\n", p.attr)

	return nil
}

func (p *LinuxProc) setCpuAffinity() {
	p.pd.mu.Lock()
	defer p.pd.mu.Unlock()

	p.setCpuAffinityL()
}

// Set the Cpu affinity of this proc according to its procd's cpu mask.
func (p *LinuxProc) setCpuAffinityL() {
	err := linuxsched.SchedSetAffinityAllTasks(p.SysPid, &p.pd.cpuMask)
	if err != nil {
		log.Printf("Error setting CPU affinity for child lambda: %v", err)
	}
}
