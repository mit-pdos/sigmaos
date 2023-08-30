package kproc

import (
	"os"
	"os/exec"
	"syscall"

	"sigmaos/config"
	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

// To run kernel procs
func RunKernelProc(parentCfg *config.SigmaConfig, p *proc.Proc, realm sp.Trealm, extra []*os.File) (*exec.Cmd, error) {
	childCfg := proc.NewChildSigmaConfig(parentCfg, p)
	p.SetSigmaConfig(childCfg)
	p.Finalize("")
	env := p.GetEnv()
	//	s, err := namedAddr.Taddrs2String()
	//	if err != nil {
	//		return nil, err
	//	}
	//	env = append(env, "SIGMAPROGRAM="+p.Program)
	env = append(env, "SIGMAROOTFS="+proc.GetSigmaRootFs())
	env = append(env, "SIGMAREALM="+realm.String())
	env = append(env, "SIGMATAG="+proc.GetBuildTag())
	cmd := exec.Command(p.Program, p.Args...)
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
