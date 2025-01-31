package file

import (
	"fmt"
	"io"
	"sigmaos/shell/shellctx"
	"sigmaos/shell/util"
	sp "sigmaos/sigmap"
)

type MvCommand struct{}

func NewMvCommand() *MvCommand {
	return &MvCommand{}
}

func (c *MvCommand) Name() string {
	return "mv"
}

func (c *MvCommand) Usage() string {
	return "mv <source> <destination>"
}

func (c *MvCommand) Execute(ctx *shellctx.ShellContext, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) bool {
	if len(args) != 2 {
		fmt.Fprintf(stderr, "Invalid number of arguments\n %v", c.Usage())
		return false
	}
	if args[0] == "--help" {
		fmt.Fprintln(stdout, c.Usage())
		return true
	}

	src := util.ResolvePath(ctx, args[0])
	dst := util.ResolvePath(ctx, args[1])

	isDir, err := ctx.Tstate.IsDir(shellctx.FILEPATH_OFFSET + src)
	if err != nil {
		fmt.Fprintf(stderr, "Error checking source: %v\n", err)
		return false
	}

	var mvErr error
	if isDir {
		mvErr = ctx.Tstate.MkDir(shellctx.FILEPATH_OFFSET+dst, sp.DMDIR|0755)
		if mvErr != nil {
			fmt.Fprintf(stderr, "Error moving: %v\n", mvErr)
			return false
		}
		_, mvErr = ctx.Tstate.MoveDirEntries(shellctx.FILEPATH_OFFSET+src, shellctx.FILEPATH_OFFSET+dst)
		if mvErr != nil {
			fmt.Fprintf(stderr, "Error moving: %v\n", mvErr)
			return false
		}
		mvErr = ctx.Tstate.RmDir(shellctx.FILEPATH_OFFSET + src)
	} else {
		mvErr = ctx.Tstate.Rename(shellctx.FILEPATH_OFFSET+src, shellctx.FILEPATH_OFFSET+dst)
	}

	if mvErr != nil {
		fmt.Fprintf(stderr, "Error moving: %v\n", mvErr)
		return false
	}

	itemType := "File"
	if isDir {
		itemType = "Directory"
	}

	_, err = fmt.Fprintf(stdout, "%s moved successfully from %s to %s\n", itemType, args[0], args[1])
	if err != nil {
		fmt.Fprintf(stderr, "error writing to stdout: %v", err)
		return false
	}

	return true
}
