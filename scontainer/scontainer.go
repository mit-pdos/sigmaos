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

type uprocCmd struct {
	cmd *exec.Cmd
}

func (upc *uprocCmd) Wait() error {
	return upc.cmd.Wait()
}

func (upc *uprocCmd) Pid() int {
	return upc.cmd.Process.Pid
}

func StartCaladanIOKernel(ch chan bool) {
	db.DPrintf(db.ALWAYS, "Lets get ready to rumble!")
	cmd := exec.Command("/home/sigmaos/junction/lib/caladan/iokerneld", []string{"ias", "noht"}...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		db.DPrintf(db.ALWAYS, "panic! Nope :(, error: %v", err)
	}
	time.Sleep(10 * time.Second)
	db.DPrintf(db.ALWAYS, "We're rumbling!")
	ch <- true
}

// Contain user procs using uproc-trampoline trampoline
func StartSigmaContainer(uproc *proc.Proc, dialproxy, useJunction bool) (*uprocCmd, error) {
	db.DPrintf(db.CONTAINER, "RunUProc dialproxy %v useJunction %v %v env %v\n", dialproxy, useJunction, uproc, os.Environ())
	var cmd *exec.Cmd
	straceProcs := proc.GetLabels(uproc.GetProcEnv().GetStrace())

	pn := binsrv.BinPath(uproc.GetVersionedProgram())
	if useJunction {
		//		cmd = exec.Command("junction_run", append([]string{"caladan_test.config", "--", pn, strconv.FormatBool(dialproxy)}, uproc.Args...)...)
		cmd = exec.Command("junction_run", append([]string{"caladan_test.config", "--", "echo", "we win"})...)
	} else {
		// Optionally strace the proc
		if straceProcs[uproc.GetProgram()] {
			cmd = exec.Command("strace", append([]string{"-D", "-f", "uproc-trampoline", uproc.GetPid().String(), pn, strconv.FormatBool(dialproxy)}, uproc.Args...)...)
		} else {
			cmd = exec.Command("uproc-trampoline", append([]string{uproc.GetPid().String(), pn, strconv.FormatBool(dialproxy)}, uproc.Args...)...)
		}
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
		CleanupUProc(uproc.GetPid())
		return nil, err
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] UProc cmd.Start %v", uproc.GetPid(), time.Since(s))
	return &uprocCmd{cmd: cmd}, nil
}

func CleanupUProc(pid sp.Tpid) {
	if err := os.RemoveAll(jailPath(pid)); err != nil {
		db.DPrintf(db.ALWAYS, "Error cleanupJail: %v", err)
	}
}

func jailPath(pid sp.Tpid) string {
	return filepath.Join(sp.SIGMAHOME, "jail", pid.String())
}
