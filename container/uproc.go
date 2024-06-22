package container

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"sigmaos/binsrv"
	db "sigmaos/debug"
	"sigmaos/linuxsched"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type uprocCmd struct {
	cmd *exec.Cmd
}

func (upc *uprocCmd) Wait() error {
	return upc.cmd.Wait()
}

func (upc *uprocCmd) Pid() int {
	return upc.cmd.Process.Pid
}

// Contain user procs using exec-uproc-rs trampoline
func StartUProc(uproc *proc.Proc, netproxy bool) (*uprocCmd, error) {
	db.DPrintf(db.CONTAINER, "RunUProc netproxy %v %v env %v\n", netproxy, uproc, os.Environ())
	var cmd *exec.Cmd
	straceProcs := proc.GetLabels(uproc.GetProcEnv().GetStrace())

	pn := binsrv.BinPath(uproc.GetVersionedProgram())
	// Optionally strace the proc
	if straceProcs[uproc.GetProgram()] {
		cmd = exec.Command("strace", append([]string{"-D", "-f", "exec-uproc-rs", uproc.GetPid().String(), pn, strconv.FormatBool(netproxy)}, uproc.Args...)...)
	} else {
		cmd = exec.Command("exec-uproc-rs", append([]string{uproc.GetPid().String(), pn, strconv.FormatBool(netproxy)}, uproc.Args...)...)
	}
	uproc.AppendEnv("PATH", "/bin:/bin2:/usr/bin:/home/sigmaos/bin/kernel")
	uproc.AppendEnv("SIGMA_EXEC_TIME", strconv.FormatInt(time.Now().UnixMicro(), 10))
	uproc.AppendEnv("SIGMA_SPAWN_TIME", strconv.FormatInt(uproc.GetSpawnTime().UnixMicro(), 10))
	uproc.AppendEnv(proc.SIGMAPERF, uproc.GetProcEnv().GetPerf())
	// uproc.AppendEnv("RUST_BACKTRACE", "1")
	cmd.Env = uproc.GetEnv()

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Set up new namespaces
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNS,
	}
	db.DPrintf(db.CONTAINER, "exec cmd %v", cmd)

	s := time.Now()
	if err := cmd.Start(); err != nil {
		db.DPrintf(db.CONTAINER, "Error start %v %v", cmd, err)
		CleanupUproc(uproc.GetPid())
		return nil, err
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] Uproc cmd.Start %v", uproc.GetPid(), time.Since(s))
	return &uprocCmd{cmd: cmd}, nil
}

func CleanupUproc(pid sp.Tpid) {
	if err := os.RemoveAll(jailPath(pid)); err != nil {
		db.DPrintf(db.ALWAYS, "Error cleanupJail: %v", err)
	}
}

func setSchedPolicy(pid int, policy linuxsched.SchedPolicy) error {
	attr, err := linuxsched.SchedGetAttr(pid)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error Getattr %v: %v", pid, err)
		return err
	}
	attr.Policy = policy
	err = linuxsched.SchedSetAttr(pid, attr)
	if err != nil {
		db.DPrintf(db.ALWAYS, "Error Setattr %v: %v", pid, err)
		return err
	}
	return nil
}

func jailPath(pid sp.Tpid) string {
	return filepath.Join(sp.SIGMAHOME, "jail", pid.String())
}
