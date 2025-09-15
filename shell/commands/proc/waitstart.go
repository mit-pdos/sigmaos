package proc

import (
	"fmt"
	"io"
	"sigmaos/shell/shellctx"
	sp "sigmaos/sigmap"
)

type WaitStartCommand struct{}

func NewWaitStartCommand() *WaitStartCommand {
	return &WaitStartCommand{}
}

func (c *WaitStartCommand) Name() string {
	return "waitstart"
}

func (c *WaitStartCommand) Usage() string {
	return "waitstart <pid>"
}

func (c *WaitStartCommand) Execute(ctx *shellctx.ShellContext, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) bool {
	if len(args) != 1 {
		fmt.Fprintf(stderr, "Invalid number of arguments\n %v", c.Usage())
		return false
	}
	if args[0] == "--help" {
		fmt.Fprintln(stdout, c.Usage())
		return true
	}
	pid := args[0]

	err := ctx.Tstate.WaitStart(sp.Tpid(pid))
	if err != nil {
		fmt.Fprintf(stderr, "Error waiting for proc to start: %v\n", err)
		return false
	}

	fmt.Fprintf(stdout, "Proc %s started\n", args[0])
	return true
}
