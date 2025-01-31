package text

import (
	"bufio"
	"fmt"
	"io"
	"sigmaos/shell/shellctx"
	"sigmaos/shell/util"
	"strconv"
	"strings"
)

type HeadCommand struct{}

func NewHeadCommand() *HeadCommand {
	return &HeadCommand{}
}

func (c *HeadCommand) Name() string {
	return "head"
}

func (c *HeadCommand) Usage() string {
	return "head [-n NUM] [filename]"
}

func (c *HeadCommand) Execute(ctx *shellctx.ShellContext, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) bool {
	if len(args) > 3 {
		fmt.Fprintf(stderr, "Invalid number of arguments\n%s\n", c.Usage())
		return false
	}

	if len(args) > 0 && args[0] == "--help" {
		fmt.Fprintln(stdout, c.Usage())
		fmt.Fprintln(stdout, "\nDisplays the first NUM lines of a file or stdin.")
		fmt.Fprintln(stdout, "If no file is specified, reads from stdin.")
		return true
	}

	numLines := 10 // Default number of lines
	filename := ""

	// Parse arguments
	for i := 0; i < len(args); i++ {
		if args[i] == "-n" {
			if i+1 < len(args) {
				var err error
				numLines, err = strconv.Atoi(args[i+1])
				if err != nil {
					fmt.Fprintf(stderr, "Invalid number of lines: %s\n", args[i+1])
					return false
				}
				i++
			} else {
				fmt.Fprintf(stderr, "Option -n requires an argument\n")
				return false
			}
		} else {
			filename = args[i]
		}
	}

	var reader io.Reader

	if filename != "" {
		resolvedPath := util.ResolvePath(ctx, filename)
		data, err := ctx.Tstate.GetFile(shellctx.FILEPATH_OFFSET + resolvedPath)
		if err != nil {
			fmt.Fprintf(stderr, "Error reading file: %v\n", err)
			return false
		}
		reader = strings.NewReader(string(data))
	} else {
		reader = stdin
	}

	scanner := bufio.NewScanner(reader)
	lineCount := 0

	for scanner.Scan() && lineCount < numLines {
		fmt.Fprintln(stdout, scanner.Text())
		lineCount++
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(stderr, "Error reading input: %v\n", err)
		return false
	}

	return true
}
