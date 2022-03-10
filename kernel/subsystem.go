package kernel

import (
	"os/exec"

	"ulambda/proc"
)

type Subsystem struct {
	cmd *exec.Cmd
	p   *proc.Proc
}

func makeSubsystem(cmd *exec.Cmd, p *proc.Proc) *Subsystem {
	return &Subsystem{cmd, p}
}

func (s *Subsystem) Monitor() {

}
