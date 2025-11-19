package proc

import (
	"fmt"
	"io"
	"sigmaos/shell/shellctx"
	sp "sigmaos/sigmap"
)

type WaitExitCommand struct{}

func NewWaitExitCommand() *WaitExitCommand {
	return &WaitExitCommand{}
}

func (c *WaitExitCommand) Name() string {
	return "waitexit"
}

func (c *WaitExitCommand) Usage() string {
	return "waitexit <pid>"
}

func (c *WaitExitCommand) Execute(ctx *shellctx.ShellContext, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) bool {
	if len(args) != 1 {
		fmt.Fprintf(stderr, "Invalid number of arguments\n %v", c.Usage())
		return false
	}
	if args[0] == "--help" {
		fmt.Fprintln(stdout, c.Usage())
		return true
	}
	pid := args[0]

	status, err := ctx.Tstate.WaitExit(sp.Tpid(pid))
	if err != nil {
		fmt.Fprintf(stderr, "Error waiting for proc to exit: %v\n", err)
		return false
	}

	fmt.Fprintf(stdout, "Proc %s exited with status: %v\n", args[0], status.IsStatusOK())
	fmt.Fprintf(stdout, "Exit message: %s\n", status.Msg())
	return true
}
