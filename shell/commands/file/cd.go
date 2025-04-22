package file

import (
	"fmt"
	"io"
	"sigmaos/shell/shellctx"
	"sigmaos/shell/util"
)

type CdCommand struct{}

func NewCdCommand() *CdCommand {
	return &CdCommand{}
}

func (c *CdCommand) Name() string {
	return "cd"
}

func (c *CdCommand) Usage() string {
	return "cd <directory>"
}

func (c *CdCommand) Execute(ctx *shellctx.ShellContext, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) bool {
	if len(args) != 1 {
		fmt.Fprintf(stderr, "Invalid number of arguments\n %v", c.Usage())
		return false
	}
	if args[0] == "--help" {
		fmt.Fprintln(stdout, c.Usage())
		return true
	}

	newDir := util.ResolvePath(ctx, args[0])

	isDir, err := ctx.Tstate.IsDir(shellctx.FILEPATH_OFFSET + newDir)
	if !isDir || err != nil {
		fmt.Fprintf(stderr, "Error changing to directory %s: %v\n", newDir, err)
		return false
	}

	ctx.CurrentDir = newDir
	return true
}
