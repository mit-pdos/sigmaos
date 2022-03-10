package kernel

import (
	"os/exec"
)

type Subsystem struct {
	cmd *exec.Cmd
	pid string
}

func makeSubsystem(cmd *exec.Cmd, pid string) *Subsystem {
	return &Subsystem{cmd, pid}
}
