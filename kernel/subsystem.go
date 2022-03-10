package kernel

import (
	"os/exec"

	"ulambda/proc"
	"ulambda/procclnt"
)

type Subsystem struct {
	*procclnt.ProcClnt
	p   *proc.Proc
	cmd *exec.Cmd
}

func makeSubsystem(pclnt *procclnt.ProcClnt, p *proc.Proc) *Subsystem {
	return &Subsystem{pclnt, p, nil}
}

func (s *Subsystem) Run(bindir string, namedAddr []string) error {
	cmd, err := s.SpawnKernelProc(s.p, bindir, namedAddr)
	if err != nil {
		return err
	}
	s.cmd = cmd
	return s.WaitStart(s.p.Pid)
}

func (s *Subsystem) Monitor() {

}
