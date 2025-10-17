package proc

import (
	"fmt"
	"io"
	"sigmaos/shell/shellctx"
	sp "sigmaos/sigmap"
)

type EvictCommand struct{}

func NewEvictCommand() *EvictCommand {
	return &EvictCommand{}
}

func (c *EvictCommand) Name() string {
	return "evict"
}

func (c *EvictCommand) Usage() string {
	return "evict <pid>"
}

func (c *EvictCommand) Execute(ctx *shellctx.ShellContext, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) bool {
	if len(args) != 1 {
		fmt.Fprintf(stderr, "Invalid number of arguments\n %v", c.Usage())
		return false
	}
	if args[0] == "--help" {
		fmt.Fprintln(stdout, c.Usage())
		return true
	}
	pid := args[0]

	err := ctx.Tstate.Evict(sp.Tpid(pid))
	if err != nil {
		fmt.Fprintf(stderr, "Error evicting proc: %v\n", err)
		return false
	}

	fmt.Fprintf(stdout, "Successfully evicted proc with PID %s\n", args[0])
	return true
}
