package kproc

import (
	"os"
	"os/exec"
	"strings"
	"syscall"

	"sigmaos/container"
	db "sigmaos/debug"
	"sigmaos/proc"
)

// To run kernel procs
func RunKernelProc(p *proc.Proc, namedAddr []string, realm string, contain bool) (*exec.Cmd, error) {
	db.DPrintf(db.KERNEL, "RunKernelProc %v %v\n", p, namedAddr)
	p.FinalizeEnv("NONE")
	env := p.GetEnv()
	env = append(env, "SIGMANAMED="+strings.Join(namedAddr, ","))
	env = append(env, "SIGMAPROGRAM="+p.Program)
	env = append(env, "SIGMAROOTFS="+proc.GetSigmaRootFs())
	// env = append(env, "PATH="+proc.GetPath())

	cmd := exec.Command(p.Program, p.Args...)
	// Create a process group ID to kill all children if necessary.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env
	if contain {
		if err := container.RunKernelContainer(cmd, realm); err != nil {
			return nil, err
		}
	} else {
		db.DPrintf(db.KERNEL, "Run kernel proc wo. container env %v\n", cmd.Env)
		if err := cmd.Start(); err != nil {
			return nil, err
		}
	}
	return cmd, nil
}
