package file

import (
	"fmt"
	"io"
	"sigmaos/shell/shellctx"
	"sigmaos/shell/util"
)

type RmCommand struct{}

func NewRmCommand() *RmCommand {
	return &RmCommand{}
}

func (c *RmCommand) Name() string {
	return "rm"
}

func (c *RmCommand) Usage() string {
	return "rm [-r] <path>"
}

func (c *RmCommand) Execute(ctx *shellctx.ShellContext, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) bool {
	if len(args) < 1 || len(args) > 2 {
		fmt.Fprintf(stderr, "Invalid number of arguments\n %v", c.Usage())
		return false
	}
	if args[0] == "--help" {
		fmt.Fprintln(stdout, c.Usage())
		return true
	}
	recursive := false
	path := ""

	if len(args) == 2 && args[0] == "-r" {
		recursive = true
		path = args[1]
	} else {
		path = args[0]
	}

	fullPath := util.ResolvePath(ctx, path)

	isDir, err := ctx.Tstate.IsDir(shellctx.FILEPATH_OFFSET + fullPath)
	if err != nil {
		fmt.Fprintf(stderr, "Error removing %v %v\n", path, err)
		return false
	}

	var rmErr error
	if isDir {
		if !recursive {
			fmt.Fprintf(stderr, "Cannot remove directory '%s': Is a directory. Use -r for recursive removal.\n", path)
			return false
		}
		rmErr = ctx.Tstate.RmDir(shellctx.FILEPATH_OFFSET + fullPath)
	} else {
		rmErr = ctx.Tstate.Remove(shellctx.FILEPATH_OFFSET + fullPath)
	}

	if rmErr != nil {
		fmt.Fprintf(stderr, "Error removing %s: %v\n", path, rmErr)
		return false
	}

	itemType := "File"
	if isDir {
		itemType = "Directory"
	}

	_, err = fmt.Fprintf(stdout, "%s %s removed successfully\n", itemType, path)
	if err != nil {
		fmt.Fprintf(stderr, "error writing to stdout: %v", err)
		return false
	}

	return true
}
