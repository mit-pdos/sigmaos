package file

import (
	"fmt"
	"io"
	"sigmaos/shell/shellctx"
	"sigmaos/shell/util"
	sp "sigmaos/sigmap"
)

type LsCommand struct{}

func NewLsCommand() *LsCommand {
	return &LsCommand{}
}

func (c *LsCommand) Name() string {
	return "ls"
}

func (c *LsCommand) Usage() string {
	return "ls [directory]"
}

func (c *LsCommand) Execute(ctx *shellctx.ShellContext, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) bool {
	var dirToList string

	if len(args) == 0 {
		dirToList = ctx.CurrentDir
	} else if len(args) == 1 {
		dirToList = util.ResolvePath(ctx, args[0])
	} else {
		fmt.Fprintf(stderr, "Invalid number of arguments\n %v", c.Usage())
		return false
	}
	sts, err := ctx.Tstate.GetDir(shellctx.FILEPATH_OFFSET + dirToList)
	if err != nil {
		fmt.Fprintf(stderr, "Error listing directory %s: %v\n", dirToList, err)
		return false
	}

	for _, filename := range sp.Names(sts) {
		fmt.Fprintln(stdout, filename)
	}

	return true
}
