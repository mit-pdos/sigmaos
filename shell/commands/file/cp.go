package file

import (
	"fmt"
	"io"
	"sigmaos/shell/shellctx"
	"sigmaos/shell/util"
)

type CpCommand struct{}

func NewCpCommand() *CpCommand {
	return &CpCommand{}
}

func (c *CpCommand) Name() string {
	return "cp"
}

func (c *CpCommand) Usage() string {
	return "cp <source> <destination>"
}

func (c *CpCommand) Execute(ctx *shellctx.ShellContext, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) bool {
	if len(args) == 1 && args[0] == "--help" {
		fmt.Fprintln(stdout, c.Usage())
		return true
	}
	if len(args) != 2 {
		fmt.Fprintf(stderr, "Invalid number of arguments\n %v", c.Usage())
		return false
	}

	src := util.ResolvePath(ctx, args[0])
	dst := util.ResolvePath(ctx, args[1])

	isDir, err := ctx.Tstate.IsDir(shellctx.FILEPATH_OFFSET + src)
	if err != nil {
		fmt.Fprintf(stderr, "Error checking source: %v\n", err)
		return false
	}

	var copyErr error
	if isDir {
		copyErr = ctx.Tstate.CopyDir(shellctx.FILEPATH_OFFSET+src, shellctx.FILEPATH_OFFSET+dst)
	} else {
		copyErr = ctx.Tstate.CopyFile(shellctx.FILEPATH_OFFSET+src, shellctx.FILEPATH_OFFSET+dst)
	}

	if copyErr != nil {
		fmt.Fprintf(stderr, "Error copying: %v\n", copyErr)
		return false
	}

	itemType := "File"
	if isDir {
		itemType = "Directory"
	}

	_, err = fmt.Fprintf(stdout, "%s copied successfully from %s to %s\n", itemType, args[0], args[1])
	if err != nil {
		fmt.Fprintf(stderr, "error writing to stdout: %v", err)
		return false
	}

	return true
}
