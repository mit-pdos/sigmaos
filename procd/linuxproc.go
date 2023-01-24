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
	"sigmaos/perf"
	"sigmaos/proc"
	"sigmaos/procclnt"
	"sigmaos/semclnt"
	"sigmaos/sigmaclnt"
)

const (
	LC_PROC_PRIORITY = 0
	BE_PROC_PRIORITY = 0
)

type LinuxProc struct {
	fs.Inode
	SysPid    int
	syspidstr string
	Env       []string
	attr      *proc.Proc
	stolen    bool
	pd        *Procd
	sclnt     *sigmaclnt.SigmaClnt
	UtilInfo  struct {
		lastUtil float64
		utime0   uint64
		stime0   uint64
		t0       time.Time
	}
}

func makeLinuxProc(pd *Procd, sclnt *sigmaclnt.SigmaClnt, a *proc.Proc, stolen bool) *LinuxProc {
	a.FinalizeEnv(pd.addr)
	p := &LinuxProc{}
	p.pd = pd
	p.sclnt = sclnt
	p.attr = a
	p.stolen = stolen
	p.Env = p.attr.GetEnv()
	db.DPrintf(db.PROCD, "Procd init: %v %v\n", p.SysPid, p.Env)
	return p
}

func (p *LinuxProc) wait(cmd *exec.Cmd) {
	defer p.pd.fs.finish(p)
	err := cmd.Wait()
	if err != nil {
		db.DPrintf(db.PROCD_ERR, "Proc %v finished with error: %v\n", p.attr, err)
		procclnt.ExitedProcd(p.sclnt.FsLib, p.attr.GetPid(), p.attr.ProcDir, p.attr.ParentDir, proc.MakeStatusErr(err.Error(), nil))
		return
	}
}

func (p *LinuxProc) run() error {
	db.DPrintf(db.PROCD, "Procd run: %v\n", p.attr)

	// Make the proc's procdir
	if err := p.pd.procclnt.MakeProcDir(p.attr.GetPid(), p.attr.ProcDir, p.attr.IsPrivilegedProc()); err != nil {
		db.DPrintf(db.PROCD_ERR, "Err procd MakeProcDir: %v\n", err)
	}

	db.DPrintf(db.PROCD_PERF, "proc %v (stolen:%v) queueing delay: %v", p.attr.GetPid(), p.stolen, time.Since(p.attr.GetSpawnTime()))
	var cmd *exec.Cmd
	if p.attr.IsPrivilegedProc() {
		cmd = exec.Command(p.attr.Program, p.attr.Args...)
		// If this is a privileged proc, wait for it to start & then mark it as
		// started.
		go func() {
			semStart := semclnt.MakeSemClnt(p.sclnt.FsLib, path.Join(p.attr.ProcDir, proc.START_SEM))
			semStart.Down()

			p.pd.mu.Lock()
			defer p.pd.mu.Unlock()
			// Sanity check that we don't start 2 of the same kernel proc on the same
			// procd.
			if _, ok := p.pd.kernelProcs[p.attr.Program]; ok && p.attr.Program != "kernel/dbd" {
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
		if err := p.pd.updm.MakeUProc(p.attr, "rootrealm"); err != nil {
			db.DPrintf(db.ALWAYS, "MakeUProc run error: %v, %v\n", p.attr, err)
			procclnt.ExitedProcd(p.sclnt.FsLib, p.attr.GetPid(), p.attr.ProcDir, p.attr.ParentDir, proc.MakeStatusErr(err.Error(), nil))
			return err
		}
		db.DPrintf(db.PROCD, "Procd ran: %v\n", p.attr)
		return nil
	}
	cmd.Env = p.Env
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	if err != nil {
		db.DPrintf(db.PROCD_ERR, "Procd run error: %v, %v\n", p.attr, err)
		procclnt.ExitedProcd(p.sclnt.FsLib, p.attr.GetPid(), p.attr.ProcDir, p.attr.ParentDir, proc.MakeStatusErr(err.Error(), nil))
		return err
	}

	// XXX GetCPUTimePid fails because proc isn't running yet; seems
	// worse now with an extra mount.

	// Take this lock to ensure we don't race with a thread rebalancing cores.
	p.pd.mu.Lock()
	p.SysPid = cmd.Process.Pid
	p.syspidstr = strconv.Itoa(p.SysPid)
	p.UtilInfo.t0 = time.Now()
	p.UtilInfo.utime0, p.UtilInfo.stime0, err = perf.GetCPUTimePid(p.syspidstr)
	p.pd.mu.Unlock()
	if err != nil {
		db.DPrintf(db.PROCD_ERR, "Procd GetCPUTimePid %v err %v\n", p.syspidstr, err)
	}
	// Nice the process.
	p.setPriority()

	p.wait(cmd)
	db.DPrintf(db.PROCD, "Procd ran: %v\n", p.attr)

	return nil
}

// Caller holds lock.
func (p *LinuxProc) getUtilL() (float64, error) {
	t1 := time.Now()
	utime1, stime1, err := perf.GetCPUTimePid(p.syspidstr)
	if err != nil {
		return 0, err
	}
	util := perf.UtilFromCPUTimeSample(p.UtilInfo.utime0, p.UtilInfo.stime0, utime1, stime1, t1.Sub(p.UtilInfo.t0).Seconds())
	if util == 0 {
		return p.UtilInfo.lastUtil, nil
	}
	p.UtilInfo.utime0 = utime1
	p.UtilInfo.stime0 = stime1
	p.UtilInfo.t0 = t1
	p.UtilInfo.lastUtil = util
	return util, nil
}

func (p *LinuxProc) setPriority() {
	var err error
	switch p.attr.GetType() {
	case proc.T_BE:
		err = linuxsched.SchedSetPriority(p.SysPid, BE_PROC_PRIORITY)
	case proc.T_LC:
		err = linuxsched.SchedSetPriority(p.SysPid, LC_PROC_PRIORITY)
	default:
		p.pd.perf.Done()
		db.DFatalf("Error unknown proc priority: %v", p.attr.GetType())
	}
	if err != nil {
		db.DPrintf(db.ALWAYS, "Couldn't set priority for %v err %v", p.attr, err)
	}
}
