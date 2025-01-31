package text

import (
	"bufio"
	"container/ring"
	"fmt"
	"io"
	"sigmaos/shell/shellctx"
	"sigmaos/shell/util"
	"strconv"
	"strings"
)

type TailCommand struct{}

func NewTailCommand() *TailCommand {
	return &TailCommand{}
}

func (c *TailCommand) Name() string {
	return "tail"
}

func (c *TailCommand) Usage() string {
	return "tail [-n NUM] [filename]"
}

func (c *TailCommand) Execute(ctx *shellctx.ShellContext, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) bool {
	if len(args) > 3 {
		fmt.Fprintf(stderr, "Invalid number of arguments\n%s\n", c.Usage())
		return false
	}

	if len(args) > 0 && args[0] == "--help" {
		fmt.Fprintln(stdout, c.Usage())
		fmt.Fprintln(stdout, "\nDisplays the last NUM lines of a file or stdin.")
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
	r := ring.New(numLines)

	for scanner.Scan() {
		r.Value = scanner.Text()
		r = r.Next()
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(stderr, "Error reading input: %v\n", err)
		return false
	}

	r.Do(func(p interface{}) {
		if p != nil {
			fmt.Fprintln(stdout, p.(string))
		}
	})

	return true
}
