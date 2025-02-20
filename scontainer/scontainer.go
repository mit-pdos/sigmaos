// This package provides StartSigmaContainer to run a proc inside a
// sigma container.
package scontainer

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/pyproxysrv"
	"sigmaos/sched/msched/proc/srv/binsrv"
	sp "sigmaos/sigmap"
)

type uprocCmd struct {
	cmd *exec.Cmd
	pps *pyproxysrv.PyProxySrv
}

func (upc *uprocCmd) Wait() error {
	return upc.cmd.Wait()
}

func (upc *uprocCmd) Pid() int {
	return upc.cmd.Process.Pid
}

// Contain user procs using uproc-trampoline trampoline
func StartSigmaContainer(uproc *proc.Proc, dialproxy bool) (*uprocCmd, error) {
	db.DPrintf(db.CONTAINER, "RunUProc dialproxy %v %v env %v\n", dialproxy, uproc, os.Environ())
	var cmd *exec.Cmd
	straceProcs := proc.GetLabels(uproc.GetProcEnv().GetStrace())

	stringProg := uproc.GetVersionedProgram()
	if uproc.GetProgram() == "python" {
		stringProg = "python"
		pythonPath, _ := uproc.LookupEnv("PYTHONPATH")
		db.DPrintf(db.CONTAINER, "PYTHONPATH: %v\n", pythonPath)
	}
	pn := binsrv.BinPath(stringProg)
	// Optionally strace the proc
	stracing := false
	if straceProcs[uproc.GetProgram()] {
		stracing = true
		db.DPrintf(db.CONTAINER, "strace %v", uproc.GetProgram())
		straceArgs := append([]string{"-D", "-f", "uproc-trampoline", uproc.GetPid().String(), pn, strconv.FormatBool(dialproxy)}, uproc.Args...)
		if uproc.GetProgram() == "python" {
			straceArgs = append([]string{"-E", "LD_PRELOAD=/tmp/python/ld_fstatat.so"}, straceArgs...)
		}
		cmd = exec.Command("strace", straceArgs...)
	} else {
		cmd = exec.Command("uproc-trampoline", append([]string{uproc.GetPid().String(), pn, strconv.FormatBool(dialproxy)}, uproc.Args...)...)
	}
	uproc.AppendEnv("PATH", "/bin:/bin2:/usr/bin:/home/sigmaos/bin/kernel")
	uproc.AppendEnv("SIGMA_EXEC_TIME", strconv.FormatInt(time.Now().UnixMicro(), 10))
	uproc.AppendEnv("SIGMA_SPAWN_TIME", strconv.FormatInt(uproc.GetSpawnTime().UnixMicro(), 10))
	uproc.AppendEnv(proc.SIGMAPERF, uproc.GetProcEnv().GetPerf())
	if uproc.GetProgram() == "python" && !stracing {
		uproc.AppendEnv("LD_PRELOAD", "/tmp/python/ld_fstatat.so")
	}
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

	// Extra setup for Python procs
	uprocCommand := &uprocCmd{cmd: cmd}
	if uproc.GetProgram() == "python" {
		bucketName, ok := uproc.LookupEnv(proc.SIGMAPYBUCKET)
		if !ok {
			err := errors.New("nil SIGMAPYBUCKET")
			db.DPrintf(db.PYPROXYSRV_ERR, "No specified AWS bucket: %v", err)
			CleanupUProc(uproc.GetPid())
			return nil, err
		}

		pps, err := pyproxysrv.NewPyProxySrv(uproc.GetProcEnv(), bucketName)
		if err != nil {
			db.DPrintf(db.PYPROXYSRV_ERR, "Error NewPyProxySrv: %v", err)
			CleanupUProc(uproc.GetPid())
			return nil, err
		}
		uprocCommand.pps = pps
	}

	s := time.Now()
	if err := cmd.Start(); err != nil {
		db.DPrintf(db.CONTAINER, "Error start %v %v", cmd, err)
		CleanupUProc(uproc.GetPid())
		return nil, err
	}
	db.DPrintf(db.SPAWN_LAT, "[%v] UProc cmd.Start %v", uproc.GetPid(), time.Since(s))
	return uprocCommand, nil
}

func CleanupUProc(pid sp.Tpid) {
	if err := os.RemoveAll(jailPath(pid)); err != nil {
		db.DPrintf(db.ALWAYS, "Error cleanupJail: %v", err)
	}
	os.RemoveAll(sp.SIGMA_PYPROXY_SOCKET)
	os.RemoveAll(sp.SIGMA_PYAPI_SOCKET)
}

func jailPath(pid sp.Tpid) string {
	return filepath.Join(sp.SIGMAHOME, "jail", pid.String())
}
