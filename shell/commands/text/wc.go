package text

import (
	"fmt"
	"io"
	"sigmaos/shell/shellctx"
	"sigmaos/shell/util"
	"strings"
)

type WcCommand struct{}

func NewWcCommand() *WcCommand {
	return &WcCommand{}
}

func (c *WcCommand) Name() string {
	return "wc"
}

func (c *WcCommand) Usage() string {
	return "wc <filename>"
}

func (c *WcCommand) Execute(ctx *shellctx.ShellContext, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) bool {
	if len(args) != 1 {
		fmt.Fprintf(stderr, "Invalid number of arguments\n%s\n", c.Usage())
		return false
	}

	if args[0] == "--help" {
		fmt.Fprintln(stdout, c.Usage())
		fmt.Fprintln(stdout, "\nCounts the number of lines, words, and bytes in the specified file.")
		fmt.Fprintln(stdout, "Output format: <lines> <words> <bytes> <filename>")
		return true
	}

	filename := args[0]
	resolvedPath := util.ResolvePath(ctx, filename)
	data, err := ctx.Tstate.GetFile(shellctx.FILEPATH_OFFSET + resolvedPath)
	if err != nil {
		fmt.Fprintf(stderr, "Error reading file '%s': %v\n", filename, err)
		return false
	}

	content := string(data)
	lines := strings.Split(content, "\n")
	words := strings.Fields(content)
	bytes := len(data)

	// Remove last empty line if file ends with a newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	fmt.Fprintf(stdout, "File: %s\n", filename)
	fmt.Fprintf(stdout, "  Lines: %d\n", len(lines))
	fmt.Fprintf(stdout, "  Words: %d\n", len(words))
	fmt.Fprintf(stdout, "  Bytes: %d\n", bytes)
	fmt.Fprintf(stdout, "\nSummary: %d %d %d %s\n", len(lines), len(words), bytes, filename)

	return true
}
