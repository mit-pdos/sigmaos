package proc

import (
	"os"
	"os/exec"
	"path"
	"strings"
	"syscall"
)

// To run kernel procs
func Run(pid, bin, name string, namedAddr []string, args []string) (*exec.Cmd, error) {
	cmd := exec.Command(path.Join(bin, name), args...)
	// Create a process group ID to kill all children if necessary.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ())
	cmd.Env = append(cmd.Env, "NAMED="+strings.Join(namedAddr, ","))
	cmd.Env = append(cmd.Env, "SIGMAPID="+pid)
	cmd.Env = append(cmd.Env, "SIGMAPIDDIR="+"pids")
	cmd.Env = append(cmd.Env, "SIGMAPARENTPID="+p.ParentPid)
	cmd.Env = append(cmd.Env, "SIGMAPARENTPIDDIR="+p.ParentPidDir)
	cmd.Env = append(cmd.Env, "SIGMAPROCDIP="+p.pd.addr)
	p.Env = env

	return cmd, cmd.Start()
}
