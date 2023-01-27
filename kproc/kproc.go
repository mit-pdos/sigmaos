package kproc

import (
	"os"
	"os/exec"
	"strings"
	"syscall"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

// To run kernel procs
func RunKernelProc(p *proc.Proc, namedAddr []string, realm sp.Trealm) (*exec.Cmd, error) {
	p.Finalize("")
	env := p.GetEnv()
	env = append(env, "SIGMANAMED="+strings.Join(namedAddr, ","))
	env = append(env, "SIGMAPROGRAM="+p.Program)
	env = append(env, "SIGMAROOTFS="+proc.GetSigmaRootFs())
	env = append(env, "SIGMAREALM="+realm.String())
	cmd := exec.Command(p.Program, p.Args...)
	// Create a process group ID to kill all children if necessary.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env

	db.DPrintf(db.KERNEL, "RunKernelProc %v %v env %v\n", p, namedAddr, env)

	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}
