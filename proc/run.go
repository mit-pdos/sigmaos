package proc

import (
	"os"
	"os/exec"
	"path"
	"strings"
	"syscall"
)

// To run kernel procs
func RunKernelProc(p *Proc, bin string, namedAddr []string) (*exec.Cmd, error) {
	env := []string{}
	env = append(p.GetEnv("NONE", "NONE")) // TODO: remove NONE
	env = append(env, "NAMED="+strings.Join(namedAddr, ","))
	env = append(env, "SIGMAPROGRAM="+p.Program)

	cmd := exec.Command(path.Join(bin, p.Program), p.Args...)
	// Create a process group ID to kill all children if necessary.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(), env...)
	return cmd, cmd.Start()
}
