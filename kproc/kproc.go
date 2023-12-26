package kproc

import (
	"os"
	"os/exec"
	"syscall"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

// To run kernel procs
func RunKernelProc(localIP sp.Thost, p *proc.Proc, extra []*os.File) (*exec.Cmd, error) {
	p.FinalizeEnv(localIP, "")
	env := p.GetEnv()
	cmd := exec.Command(p.GetProgram(), p.Args...)
	// Create a process group ID to kill all children if necessary.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.ExtraFiles = extra
	cmd.Env = env
	db.DPrintf(db.KERNEL, "RunKernelProc %v env %v", p, env)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}
