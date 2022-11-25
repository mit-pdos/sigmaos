package procd

import (
	"os"
	"os/exec"
	"path"
	"strconv"
	"time"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/linuxsched"
	"sigmaos/namespace"
	np "sigmaos/ninep"
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/semclnt"
)

const (
	LC_PROC_PRIORITY = 0
	BE_PROC_PRIORITY = 0
)

type LinuxProc struct {
	fs.Inode
	SysPid       int
	syspidstr    string
	Env          []string
	coresAlloced proc.Tcore
	attr         *proc.Proc
	stolen       bool
	pd           *Procd
	UtilInfo     struct {
		utime0 uint64
		stime0 uint64
		t0     time.Time
	}
}

func makeLinuxProc(pd *Procd, a *proc.Proc, stolen bool) *LinuxProc {
	a.FinalizeEnv(pd.addr)
	p := &LinuxProc{}
	p.pd = pd
	p.attr = a
	p.stolen = stolen
	db.DPrintf("PROCD", "Procd init: %v\n", p)
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

	db.DPrintf("PROCD_PERF", "proc %v (stolen:%v) queueing delay: %v", p.attr.Pid, p.stolen, time.Since(p.attr.SpawnTime))
	var cmd *exec.Cmd
	if p.attr.IsPrivilegedProc() {
		cmd = exec.Command(path.Join(np.PRIVILEGED_BIN, p.attr.Program), p.attr.Args...)
		// If this is a privileged proc, wait for it to start & then mark it as
		// started.
		go func() {
			semStart := semclnt.MakeSemClnt(p.pd.FsLib, path.Join(p.attr.ProcDir, proc.START_SEM))
			semStart.Down()

			p.pd.Lock()
			defer p.pd.Unlock()
			// Sanity check that we don't start 2 of the same kernel proc on the same
			// procd.
			if _, ok := p.pd.kernelProcs[p.attr.Program]; ok {
				db.DPrintf(db.ALWAYS, "Double-spawned %v on procd %v", p.attr.Program, p.pd.addr)
			}
			// Mark that we've spawned a new kernel proc.
			p.pd.kernelProcs[p.attr.Program] = true
			// If we have spawned all kernel procs, then kernel init is done.
			if len(p.pd.kernelProcs) == 3 {
				p.pd.kernelInitDone = true
			}
		}()
	} else {
		cmd = exec.Command(path.Join(np.UXROOT, p.pd.realmbin, p.attr.Program), p.attr.Args...)
		namespace.SetupProc(cmd)
	}
	cmd.Env = p.Env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		db.DPrintf("PROCD_ERR", "Procd run error: %v, %v\n", p.attr, err)
		p.pd.procclnt.ExitedProcd(p.attr.Pid, p.attr.ProcDir, p.attr.ParentDir, proc.MakeStatusErr(err.Error(), nil))
		return err
	}

	// Take this lock to ensure we don't race with a thread rebalancing cores.
	p.pd.Lock()
	p.SysPid = cmd.Process.Pid
	p.syspidstr = strconv.Itoa(p.SysPid)
	p.UtilInfo.t0 = time.Now()
	p.UtilInfo.utime0, p.UtilInfo.stime0 = perf.GetCPUTimePid(p.syspidstr)
	p.pd.Unlock()

	// XXX May want to start the process with a certain affinity (using taskset)
	// instead of setting the affinity after it starts
	p.setCpuAffinity()
	// Nice the process.
	p.setPriority()

	p.wait(cmd)
	db.DPrintf("PROCD", "Procd ran: %v\n", p.attr)

	return nil
}

func (p *LinuxProc) setCpuAffinity() {
	p.pd.Lock()
	defer p.pd.Unlock()

	p.setCpuAffinityL()
}

// Caller holds lock.
func (p *LinuxProc) getUtilL() float64 {
	t1 := time.Now()
	utime1, stime1 := perf.GetCPUTimePid(p.syspidstr)
	util := perf.UtilFromCPUTimeSample(p.UtilInfo.utime0, p.UtilInfo.stime0, utime1, stime1, t1.Sub(p.UtilInfo.t0).Seconds())
	p.UtilInfo.utime0 = utime1
	p.UtilInfo.stime0 = stime1
	p.UtilInfo.t0 = t1
	return util
}

// Set the Cpu affinity of this proc according to its procd's cpu mask.
func (p *LinuxProc) setCpuAffinityL() {
	err := linuxsched.SchedSetAffinityAllTasks(p.SysPid, &p.pd.cpuMask)
	if err != nil {
		db.DPrintf("PROCD_ERR", "Error setting CPU affinity for child lambda: %v", err)
	}
}

func (p *LinuxProc) setPriority() {
	var err error
	switch p.attr.Type {
	case proc.T_BE:
		err = linuxsched.SchedSetPriority(p.SysPid, BE_PROC_PRIORITY)
	case proc.T_LC:
		err = linuxsched.SchedSetPriority(p.SysPid, LC_PROC_PRIORITY)
	default:
		db.DFatalf("Error unknown proc priority: %v", p.attr.Type)
	}
	if err != nil {
		db.DPrintf(db.ALWAYS, "Couldn't set priority for %v err %v", p.attr, err)
	}
}
