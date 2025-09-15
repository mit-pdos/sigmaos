package text

import (
	"fmt"
	"io"
	"sigmaos/shell/shellctx"
	"strings"
)

type EchoCommand struct{}

func NewEchoCommand() *EchoCommand {
	return &EchoCommand{}
}

func (c *EchoCommand) Name() string {
	return "echo"
}

func (c *EchoCommand) Usage() string {
	return "echo [args...]"
}

func (c *EchoCommand) Execute(ctx *shellctx.ShellContext, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) bool {
	// Join all arguments with a space and print them
	output := strings.Join(args, " ")
	fmt.Fprintln(stdout, output)
	return true
}
