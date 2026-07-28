package container

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/sched/msched/proc/srv/binsrv"
	"sigmaos/scontainer"
	"sigmaos/util/linux/mem"
	"sigmaos/util/perf"
)

const (
	PYTHON_BIN_DIR = "/home/sigmaos/bin/python"
)

type pyCmd struct {
	cmd *exec.Cmd
}

func (pc *pyCmd) Pid() int {
	return pc.cmd.Process.Pid
}

func (pc *pyCmd) GetPSS() (proc.Tmem, error) {
	return mem.GetAggregatePSS(pc.cmd.Process.Pid)
}

func (pc *pyCmd) Wait() error {
	return pc.cmd.Wait()
}

// StartPythonContainer runs a Python proc through uproc-trampoline, providing
// the same PID/UTS/mount namespace isolation as a sigma container. The
// trampoline execs /usr/bin/python3 with scriptPath as its first argument.
func StartPythonContainer(uproc *proc.Proc, dialproxy bool) (*pyCmd, error) {
	// The trampoline pivot_roots into the jail shared by every proc on this
	// node; make sure it exists.
	if err := scontainer.EnsureJail(); err != nil {
		db.DPrintf(db.CONTAINER, "Error EnsureJail: %v", err)
		return nil, err
	}
	scriptPath := filepath.Join(binsrv.BINFSMNT, uproc.GetVersionedProgram())

	straceProcs := proc.GetLabels(uproc.GetProcEnv().GetStrace())
	valgrindProcs := proc.GetLabels(uproc.GetProcEnv().GetValgrind())

	trampolineArgs := []string{
		uproc.GetPid().String(),
		"/usr/bin/python3",
		strconv.FormatBool(dialproxy),
		scriptPath,
	}
	trampolineArgs = append(trampolineArgs, uproc.GetArgs()...)

	var cmd *exec.Cmd
	if straceProcs[uproc.GetProgram()] {
		args := []string{"--absolute-timestamps", "--absolute-timestamps=precision:us", "--syscall-times=us", "-D", "-f", "uproc-trampoline"}
		if strings.Contains(uproc.GetProgram(), "cpp") {
			args = append([]string{"--signal=!SIGSEGV"}, args...)
		}
		args = append(args, trampolineArgs...)
		cmd = exec.Command("strace", args...)
	} else if valgrindProcs[uproc.GetProgram()] {
		cmd = exec.Command("valgrind", append([]string{"--trace-children=yes", "uproc-trampoline"}, trampolineArgs...)...)
	} else {
		cmd = exec.Command("uproc-trampoline", trampolineArgs...)
	}

	// Signal to the trampoline that Python-specific mounts are needed.
	uproc.AppendEnv("SIGMA_PYTHON_PROC", "1")
	uproc.AppendEnv("PATH", "/bin:/usr/bin:/home/sigmaos/bin/kernel")
	uproc.AppendEnv("PYTHONPATH", "/home/sigmaos/python")
	uproc.AppendEnv("SIGMA_EXEC_TIME", strconv.FormatInt(time.Now().UnixMicro(), 10))
	b, err := time.Now().MarshalText()
	if err != nil {
		db.DFatalf("Error marshal timestamp: %v", err)
	}
	uproc.AppendEnv("SIGMA_EXEC_TIME_PB", string(b))
	uproc.AppendEnv("SIGMA_SPAWN_TIME", strconv.FormatInt(uproc.GetSpawnTime().UnixMicro(), 10))
	uproc.AppendEnv(proc.SIGMAPERF, uproc.GetProcEnv().GetPerf())
	cmd.Env = uproc.GetEnv()

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNS,
	}

	db.DPrintf(db.CONTAINER, "StartPythonContainer %v args %v", scriptPath, trampolineArgs)

	s := time.Now()
	if err := cmd.Start(); err != nil {
		db.DPrintf(db.CONTAINER, "StartPythonContainer err %v: %v", cmd, err)
		return nil, err
	}
	perf.LogSpawnLatency("StartPythonContainer cmd.Start", uproc.GetPid(), uproc.GetSpawnTime(), s)
	return &pyCmd{cmd: cmd}, nil
}

// PythonBinPath returns the local filesystem path for a Python proc's script,
// used when the script is staged in PYTHON_BIN_DIR rather than served via BINFS.
func PythonBinPath(program string) string {
	return filepath.Join(PYTHON_BIN_DIR, program)
}
