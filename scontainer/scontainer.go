// This package provides StartSigmaContainer to run a proc inside a
// sigma container.
package scontainer

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
	sp "sigmaos/sigmap"
	"sigmaos/util/perf"
	"sigmaos/wasmruntime"
)

// UprocRunner abstracts both process-based and thread-based uproc execution
type UprocRunner interface {
	Wait() error
	Pid() int
}

// uprocCmd implements UprocRunner for process-based execution
type uprocCmd struct {
	cmd *exec.Cmd
}

func (upc *uprocCmd) Wait() error {
	return upc.cmd.Wait()
}

func (upc *uprocCmd) Pid() int {
	return upc.cmd.Process.Pid
}

// uprocWasm implements UprocRunner for WASM thread-based execution
type uprocWasm struct {
	wt *wasmruntime.WasmThread
}

func (uw *uprocWasm) Wait() error {
	return uw.wt.Wait()
}

func (uw *uprocWasm) Pid() int {
	return uw.wt.Pid()
}

// Contain user procs using uproc-trampoline or in-process WASM runtime
func StartSigmaContainer(uproc *proc.Proc, dialproxy bool, wasmrt *wasmruntime.Runtime) (UprocRunner, error) {
	db.DPrintf(db.CONTAINER, "RunUProc dialproxy %v %v env %v\n", dialproxy, uproc, os.Environ())
	var cmd *exec.Cmd
	straceProcs := proc.GetLabels(uproc.GetProcEnv().GetStrace())
	valgrindProcs := proc.GetLabels(uproc.GetProcEnv().GetValgrind())

	pn := binsrv.BinPath(uproc.GetVersionedProgram())

	uproc.AppendEnv("PATH", "/bin:/bin2:/usr/bin:/home/sigmaos/bin/kernel")
	uproc.AppendEnv("SIGMA_EXEC_TIME", strconv.FormatInt(time.Now().UnixMicro(), 10))
	b, err := time.Now().MarshalText()
	if err != nil {
		db.DFatalf("Error marshal timestamp pb: %v", err)
	}
	uproc.AppendEnv("SIGMA_EXEC_TIME_PB", string(b))
	uproc.AppendEnv("SIGMA_SPAWN_TIME", strconv.FormatInt(uproc.GetSpawnTime().UnixMicro(), 10))
	uproc.AppendEnv(proc.SIGMAPERF, uproc.GetProcEnv().GetPerf())
	// uproc.AppendEnv("RUST_BACKTRACE", "1")

	// Check if this is a WASM module - if so, use in-process runtime
	// CR nmassri: this is a sketchy way, we aren't running wasm-runtime I just need to escape /mnt/binfs
	// spawn wasm-runtime /home/sigmaos/bin/wasm/hello-world-wasm.wasm
	if pn == "/mnt/binfs/wasm-runtime-v1.0" {
		db.DPrintf(db.CONTAINER, "[%v] Detected WASM module, using in-process runtime: %s", uproc.GetPid(), pn)

		wasmPath := uproc.Args[0]
		db.DPrintf(db.CONTAINER, "[%v] WASM file path from args: %s", uproc.GetPid(), wasmPath)
		uproc.Args = uproc.Args[1:]

		wt, err := wasmrt.SpawnInstance(uproc, wasmPath)
		if err != nil {
			db.DPrintf(db.CONTAINER, "[%v] Error spawning WASM instance: %v", uproc.GetPid(), err)
			CleanupUProc(uproc.GetPid())
			return nil, err
		}
		perf.LogSpawnLatency("StartSigmaContainer WASM spawn", uproc.GetPid(), uproc.GetSpawnTime(), time.Now())
		return &uprocWasm{wt: wt}, nil
	}
	// Traditional process-based execution for native binaries
	// Optionally strace the proc
	if straceProcs[uproc.GetProgram()] {
		args := []string{"--absolute-timestamps", "--absolute-timestamps=precision:us", "--syscall-times=us", "-D", "-f", "uproc-trampoline", uproc.GetPid().String(), pn, strconv.FormatBool(dialproxy)}
		if strings.Contains(uproc.GetProgram(), "cpp") {
			// Don't catch SIGSEGV for C++ programs, as this can lead to an infinite
			// strace output loop.
			args = append([]string{"--signal=!SIGSEGV"}, args...)
		}
		args = append(args, uproc.Args...)
		cmd = exec.Command("strace", args...)
	} else if valgrindProcs[uproc.GetProgram()] {
		cmd = exec.Command("valgrind", append([]string{"--trace-children=yes", "uproc-trampoline", uproc.GetPid().String(), pn, strconv.FormatBool(dialproxy)}, uproc.Args...)...)
	} else {
		// Normal uproc-trampoline execution
		cmd = exec.Command("uproc-trampoline", append([]string{uproc.GetPid().String(), pn, strconv.FormatBool(dialproxy)}, uproc.Args...)...)
	}
	// Set the environment for the process command
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
	perf.LogSpawnLatency("StartSigmaContainer cmd.Start", uproc.GetPid(), uproc.GetSpawnTime(), s)
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
