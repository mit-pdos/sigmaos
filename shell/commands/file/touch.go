package file

import (
	"fmt"
	"io"
	"sigmaos/shell/shellctx"
	"sigmaos/shell/util"
	sp "sigmaos/sigmap"
)

type TouchCommand struct{}

func NewTouchCommand() *TouchCommand {
	return &TouchCommand{}
}

func (c *TouchCommand) Name() string {
	return "touch"
}

func (c *TouchCommand) Usage() string {
	return "touch <filename> [content]"
}

func (c *TouchCommand) Execute(ctx *shellctx.ShellContext, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) bool {
	if len(args) < 1 || len(args) > 2 {
		fmt.Fprintf(stderr, "Invalid number of arguments\n %v", c.Usage())
		return false
	}
	if args[0] == "--help" {
		fmt.Fprintln(stdout, c.Usage())
		return true
	}

	filename := util.ResolvePath(ctx, args[0])
	var content []byte
	if len(args) == 2 {
		content = []byte(args[1])
	}

	_, err := ctx.Tstate.PutFile(shellctx.FILEPATH_OFFSET+filename, 0777, sp.OWRITE, content)
	if err != nil {
		fmt.Fprintf(stderr, "Error writing file: %v\n", err)
		return false
	}

	_, err = fmt.Fprintf(stdout, "File %s created or updated successfully\n", args[0])
	if err != nil {
		fmt.Fprintf(stderr, "error writing to stdout: %v", err)
		return false
	}

	return true
}
