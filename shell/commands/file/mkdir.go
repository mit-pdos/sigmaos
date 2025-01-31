package file

import (
	"fmt"
	"io"
	"sigmaos/shell/shellctx"
	"sigmaos/shell/util"
	sp "sigmaos/sigmap"
)

type MkdirCommand struct{}

func NewMkdirCommand() *MkdirCommand {
	return &MkdirCommand{}
}

func (c *MkdirCommand) Name() string {
	return "mkdir"
}

func (c *MkdirCommand) Usage() string {
	return "mkdir <directory_name>"
}

func (c *MkdirCommand) Execute(ctx *shellctx.ShellContext, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) bool {
	if len(args) != 1 {
		fmt.Fprintf(stderr, "Invalid number of arguments\n %v", c.Usage())
		return false
	}
	if args[0] == "--help" {
		fmt.Fprintln(stdout, c.Usage())
		return true
	}
	dirPath := util.ResolvePath(ctx, args[0])

	err := ctx.Tstate.MkDir(shellctx.FILEPATH_OFFSET+dirPath, sp.DMDIR|0755)
	if err != nil {
		fmt.Fprintf(stderr, "Error creating directory: %v\n", err)
		return false
	}

	_, err = fmt.Fprintf(stdout, "Directory created successfully: %s\n", args[0])
	if err != nil {
		fmt.Fprintf(stderr, "error writing to stdout: %v", err)
		return false
	}

	return true
}
