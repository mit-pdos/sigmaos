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
	env := []string{}
	env = append(env, "NAMED="+strings.Join(namedAddr, ","))
	env = append(env, SIGMAPID+"="+pid)

	cmd := exec.Command(path.Join(bin, name), args...)
	// Create a process group ID to kill all children if necessary.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), env...)
	return cmd, cmd.Start()
}
