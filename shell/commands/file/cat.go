package file

import (
	"fmt"
	"io"
	"sigmaos/shell/shellctx"
	"sigmaos/shell/util"
)

type CatCommand struct{}

func NewCatCommand() *CatCommand {
	return &CatCommand{}
}

func (c *CatCommand) Name() string {
	return "cat"
}

func (c *CatCommand) Usage() string {
	return "cat <filename>"
}

func (c *CatCommand) Execute(ctx *shellctx.ShellContext, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) bool {
	if len(args) != 1 {
		fmt.Fprintf(stderr, "Invalid number of arguments\n %v", c.Usage())
		return false
	}
	if args[0] == "--help" {
		fmt.Fprintln(stdout, c.Usage())
		return true
	}

	filename := util.ResolvePath(ctx, args[0])
	data, err := ctx.Tstate.GetFile(shellctx.FILEPATH_OFFSET + filename)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading file: %v\n", err)
		return false
	}

	_, err = fmt.Fprint(stdout, string(data))
	if err != nil {
		fmt.Fprintf(stderr, "error writing to stdout: %v", err)
		return false
	}

	return true
}
