package file

import (
	"fmt"
	"io"
	"sigmaos/shell/shellctx"
)

type PwdCommand struct{}

func NewPwdCommand() *PwdCommand {
	return &PwdCommand{}
}

func (c *PwdCommand) Name() string {
	return "pwd"
}

func (c *PwdCommand) Usage() string {
	return "pwd"
}

func (c *PwdCommand) Execute(ctx *shellctx.ShellContext, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) bool {
	if len(args) != 0 {
		fmt.Fprintf(stderr, "Invalid number of arguments\n %v", c.Usage())
		return false
	}

	_, err := fmt.Fprintln(stdout, ctx.CurrentDir)
	if err != nil {
		fmt.Fprintf(stderr, "error writing to stdout: %v", err)
		return false
	}

	return true
}
