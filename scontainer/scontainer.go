// This package provides StartSigmaContainer to run a proc inside a
// sigma container.
package scontainer

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sched/msched/proc/srv/binsrv"
	sp "sigmaos/sigmap"
)

const (
	LAZY = true
)

type uprocCmd struct {
	cmd *exec.Cmd
	ino uint64
}

func (upc *uprocCmd) Wait() error {
	return upc.cmd.Wait()
}

func (upc *uprocCmd) Pid() int {
	return upc.cmd.Process.Pid
}

func (upc *uprocCmd) Ino() uint64 {
	return upc.ino
}

// Contain user procs using exec-uproc-rs trampoline
func StartSigmaContainer(uproc *proc.Proc, dialproxy bool) (*uprocCmd, error) {
	db.DPrintf(db.CONTAINER, "RunUProc dialproxy %v %v env %v\n", dialproxy, uproc, os.Environ())
	var cmd *exec.Cmd
	straceProcs := proc.GetLabels(uproc.GetProcEnv().GetStrace())

	pn := binsrv.BinPath(uproc.GetVersionedProgram())
	db.DPrintf(db.CONTAINER, "StartUProc %q dialproxy %v %v env %v\n", pn, dialproxy, uproc, os.Environ())

	// Optionally strace the proc
	if straceProcs[uproc.GetProgram()] {
		cmd = exec.Command("strace", append([]string{"-D", "-f", "exec-uproc-rs", uproc.GetPid().String(), pn, strconv.FormatBool(dialproxy)}, uproc.Args...)...)
	} else {
		cmd = exec.Command("exec-uproc-rs", append([]string{uproc.GetPid().String(), pn, strconv.FormatBool(dialproxy)}, uproc.Args...)...)
	}
	uproc.AppendEnv("PATH", "/bin:/bin2:/usr/bin:/home/sigmaos/bin/kernel")
	uproc.AppendEnv("SIGMA_EXEC_TIME", strconv.FormatInt(time.Now().UnixMicro(), 10))
	uproc.AppendEnv("SIGMA_SPAWN_TIME", strconv.FormatInt(uproc.GetSpawnTime().UnixMicro(), 10))
	uproc.AppendEnv(proc.SIGMAPERF, uproc.GetProcEnv().GetPerf())
	// uproc.AppendEnv("RUST_BACKTRACE", "1")
	cmd.Env = uproc.GetEnv()

	f, err := os.Create("/tmp/sigmaos-perf/log-proc.txt")
	if err != nil {
		db.DFatalf("Error creating %v\n", err)
	}

	rdr, _, err := newSocketPair()
	if err != nil {
		db.DFatalf("unixPair %v\n", err)
	}

	cmd.Stdout = f
	cmd.Stderr = f
	cmd.ExtraFiles = []*os.File{rdr}

	ino := ino(rdr)

	// Set up new namespaces
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
		Cloneflags: syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNS,
	}
	db.DPrintf(db.CONTAINER, "exec cmd %v", cmd)

	s := time.Now()
	if err := cmd.Start(); err != nil {
		db.DPrintf(db.CONTAINER, "Error start %v %v", cmd, err)
		CleanupUProc(uproc.GetPid())
		return nil, err
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] Uproc cmd.Start %v", uproc.GetPid(), time.Since(s))
	db.DPrintf(db.SPAWN_LAT, "[%v] UProc cmd.Start %v", uproc.GetPid(), time.Since(s))

	rdr.Close()

	return &uprocCmd{cmd: cmd, ino: ino}, nil
}

func CleanupUProc(pid sp.Tpid) {
	if err := os.RemoveAll(jailPath(pid)); err != nil {
		db.DPrintf(db.ALWAYS, "Error cleanupJail: %v", err)
	}
}

func jailPath(pid sp.Tpid) string {
	return filepath.Join(sp.SIGMAHOME, "jail", pid.String())
}
