package proc

import (
	"fmt"
	"io"
	"sigmaos/shell/shellctx"
	"sigmaos/proc"
)

type SpawnCommand struct{}

func NewSpawnCommand() *SpawnCommand {
	return &SpawnCommand{}
}

func (c *SpawnCommand) Name() string {
	return "spawn"
}

func (c *SpawnCommand) Usage() string {
	return "spawn <proc_name> [args...]"
}

func (c *SpawnCommand) Execute(ctx *shellctx.ShellContext, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) bool {
	if len(args) < 1 {
		fmt.Fprintf(stderr, "Invalid number of arguments\n %v", c.Usage())
		return false
	}
	if args[0] == "--help" {
		fmt.Fprintln(stdout, c.Usage())
		return true
	}
	procName := args[0]
	procArgs := args[1:]

	p := proc.NewProc(procName, procArgs)
	err := ctx.Tstate.Spawn(p)
	if err != nil {
		fmt.Fprintf(stderr, "Error spawning proc: %v\n", err)
		return false
	}
	p.GetRealm()
	fmt.Fprintf(stdout, "Spawned proc with PID and realm: %v %v\n", p.GetPid(), p.GetRealm())
	return false
}
