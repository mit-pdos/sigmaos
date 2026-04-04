// This package provides StartSigmaContainer to run a proc inside a
// sigma container.
package scontainer

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/pyenv"
	pyenvclnt "sigmaos/pyenv/clnt"
	"sigmaos/sched/msched/proc/srv/binsrv"
	"sigmaos/scontainer/python"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/util/linux/mem"
	"sigmaos/util/perf"

	"golang.org/x/sys/unix"
)

// UProcCmd is a handle for a running user proc inside a sigma container.
// It is returned by StartSigmaContainer.
type UProcCmd struct {
	uproc      *proc.Proc
	cmd        *exec.Cmd
	lockHandle pyenvclnt.LockHandle
}

func (upc *UProcCmd) Wait() error {
	return upc.cmd.Wait()
}

func (upc *UProcCmd) Pid() int {
	return upc.cmd.Process.Pid
}

func (upc *UProcCmd) GetPSS() (proc.Tmem, error) {
	return mem.GetAggregatePSS(upc.cmd.Process.Pid)
}

func (upc *UProcCmd) Kill() error {
	if upc == nil || upc.cmd == nil || upc.cmd.Process == nil {
		return nil
	}
	return upc.cmd.Process.Kill()
}

var protoJailOnce sync.Once

// Contain user procs using uproc-trampoline trampoline
func StartSigmaContainer(uproc *proc.Proc, dialproxy bool, sc *sigmaclnt.SigmaClnt, pyenvClnt *pyenvclnt.PyEnvClnt) (*UProcCmd, error) {
	protoJailOnce.Do(func() {
		db.DPrintf(db.CONTAINER, "Setting up proto jail")
		if _, err := SetupProtoJail(); err != nil {
			db.DFatalf("Failed to set up proto jail: %v", err)
		}
		db.DPrintf(db.CONTAINER, "Did setup proto jail")
	})

	db.DPrintf(db.CONTAINER, "RunUProc scontainer dialproxy %v %v env %v\n", dialproxy, uproc, os.Environ())

	uprocCmd := &UProcCmd{uproc: uproc, cmd: nil, lockHandle: 0}

	straceProcs := proc.GetLabels(uproc.GetProcEnv().GetStrace())
	valgrindProcs := proc.GetLabels(uproc.GetProcEnv().GetValgrind())

	pn := binsrv.BinPath(uproc.GetVersionedProgram())

	var pythonVersion *pyenv.PythonVersion
	if version, ok := uproc.LookupEnv("SIGMA_PYTHON_VERSION"); ok {
		pythonVersion = pyenv.GetPythonVersion(version)
		if pythonVersion == nil {
			err := fmt.Errorf("unsupported Python version: %s", version)
			db.DPrintf(db.CONTAINER, "%v", err)
			return nil, err
		}
	}
	isPythonProc := pythonVersion != nil
	if isPythonProc {
		startPythonSetup := time.Now()
		pythonPath := pythonVersion.PythonPath()

		// The scontainer proto jail has all python persions mounted at
		// /tmp/python/<python-version>/python.
		pn = "/tmp/python/" + pythonVersion.Version() + "/python"

		if pythonFile, argIndex, err := python.GetPythonFileArg(uproc.Args); err == nil {
			db.DPrintf(db.CONTAINER, "pythonFile %v\n", pythonFile)

			// We need to prefix the python file path with where the python
			// interpreter can retrieve it inside the jail.
			uproc.Args[argIndex] = filepath.Join("/tmp/python/pyproc", pythonFile)

			// Set up python environment based on pylock file (if present)
			if pylockPath, err := python.GetPylockPath("/home/sigmaos/bin/kernel/pyproc", pythonFile); err == nil {
				db.DPrintf(db.CONTAINER, "setting up python site-packages from %v", pylockPath)

				additionalPythonPath, lockHandle, err := python.SetupSitePackages(
					uproc,
					pythonVersion,
					pylockPath,
					pyenvClnt,
				)

				if err != nil {
					err = fmt.Errorf("setting up python site-packages failed: %w", err)
					db.DPrintf(db.CONTAINER, "%v", err)
					return nil, err
				}

				if additionalPythonPath != "" {
					pythonPath = pythonPath + ":" + additionalPythonPath
				}

				// Store the lock handle for cleanup
				uprocCmd.lockHandle = lockHandle
			} else {
				db.DPrintf(db.CONTAINER, "No pylock.toml file found\n")
			}
		} else {
			db.DPrintf(db.CONTAINER, "No python file argument found\n")
		}

		db.DPrintf(db.CONTAINER, "PYTHONPATH: %v\n", pythonPath)
		uproc.AppendEnv("PYTHONPATH", pythonPath)
		perf.LogSpawnLatency("StartSigmaContainer python setup", uproc.GetPid(), uproc.GetSpawnTime(), startPythonSetup)
	}

	// Optionally strace the proc
	if straceProcs[uproc.GetProgram()] {
		args := []string{"--absolute-timestamps", "--absolute-timestamps=precision:us", "--syscall-times=us", "-D", "-f", "uproc-trampoline", uproc.GetPid().String(), pn, strconv.FormatBool(dialproxy)}
		if strings.Contains(uproc.GetProgram(), "cpp") || isPythonProc {
			// Don't catch SIGSEGV for C++ programs, as this can lead to an infinite
			// strace output loop.
			args = append([]string{"--signal=!SIGSEGV"}, args...)
		}
		args = append(args, uproc.Args...)
		uprocCmd.cmd = exec.Command("strace", args...)
	} else if valgrindProcs[uproc.GetProgram()] {
		uprocCmd.cmd = exec.Command("valgrind", append([]string{"--trace-children=yes", "uproc-trampoline", uproc.GetPid().String(), pn, strconv.FormatBool(dialproxy)}, uproc.Args...)...)
	} else {
		uprocCmd.cmd = exec.Command("uproc-trampoline", append([]string{uproc.GetPid().String(), pn, strconv.FormatBool(dialproxy)}, uproc.Args...)...)
	}
	uproc.AppendEnv("PATH", "/bin:/bin2:/usr/bin:/home/sigmaos/bin/kernel")
	uproc.AppendEnv("SIGMA_EXEC_TIME", strconv.FormatInt(time.Now().UnixMicro(), 10))
	b, err := time.Now().MarshalText()
	if err != nil {
		db.DFatalf("Error marshal timestamp pb: %v", err)
	}
	uproc.AppendEnv("SIGMA_EXEC_TIME_PB", string(b))
	uproc.AppendEnv("SIGMA_SPAWN_TIME", strconv.FormatInt(uproc.GetSpawnTime().UnixMicro(), 10))
	uproc.AppendEnv(proc.SIGMAPERF, uproc.GetProcEnv().GetPerf())
	uproc.AppendEnv("RUST_BACKTRACE", "full")
	uprocCmd.cmd.Env = uproc.GetEnv()

	uprocCmd.cmd.Stdout = os.Stdout
	uprocCmd.cmd.Stderr = os.Stderr

	// Set up new namespaces
	uprocCmd.cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUTS | syscall.CLONE_NEWPID,
	}
	db.DPrintf(db.CONTAINER, "exec cmd %v", uprocCmd.cmd)

	s := time.Now()
	if err := uprocCmd.cmd.Start(); err != nil {
		db.DPrintf(db.CONTAINER, "Error start %v %v", uprocCmd.cmd, err)
		CleanupUProc(uprocCmd, pyenvClnt)
		return nil, err
	}
	perf.LogSpawnLatency("StartSigmaContainer cmd.Start", uproc.GetPid(), uproc.GetSpawnTime(), s)
	return uprocCmd, nil
}

func CleanupUProc(uprocCmd *UProcCmd, pyenvClnt *pyenvclnt.PyEnvClnt) {
	// Release Python package locks
	if uprocCmd.lockHandle != 0 {
		if err := pyenvClnt.ReleaseLocks(uprocCmd.lockHandle); err != nil {
			db.DPrintf(db.CONTAINER, "Error releasing Python package locks: %v", err)
		}
	}
}

// Spawn the [cmd/kernel/scontainer] command to create a new mount namespace that
// contains the scontainer jail structure. Returns a path to the mount namespace file
// that can be used by the uproc-trampoline to join the jail.
func SetupProtoJail() (string, error) {
	jailPath := filepath.Join(sp.SIGMAHOME, "jail/proto")
	jailMntNsPath := filepath.Join(sp.SIGMAHOME, "jail/mntns")
	cmd := exec.Command("/home/sigmaos/bin/kernel/scontainer", jailPath)
	cmd.Stderr = os.Stderr

	// Capture stdout
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("Failed to capture stdout: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("Failed to start scontainer proto jail binary: %v", err)
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "ok") {
			break
		}
	}

	if err := scanner.Err(); err != nil {
		cmd.Process.Kill()
		return "", fmt.Errorf("Error reading stdout: %v", err)
	}

	go func() {
		if err := cmd.Wait(); err != nil {
			db.DPrintf(db.ALWAYS, "Proto jail process exited with error: %v", err)
		} else {
			db.DPrintf(db.ALWAYS, "Proto jail process exited")
		}
	}()

	// Capture mount namespace
	f, err := os.OpenFile(jailMntNsPath, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		cmd.Process.Kill()
		return "", fmt.Errorf("error creating file %v: %w", jailMntNsPath, err)
	}
	f.Close()

	procMntNsPath := fmt.Sprintf("/proc/%d/ns/mnt", cmd.Process.Pid)
	fmt.Printf("Mounting proto jail mount namespace from %v to %v\n", procMntNsPath, jailMntNsPath)

	err = unix.Mount(procMntNsPath, jailMntNsPath, "", unix.MS_BIND, "")
	if err != nil {
		cmd.Process.Kill()
		return "", fmt.Errorf("Failed to mount mount namespace: %v", err)
	}

	return jailMntNsPath, nil
}
