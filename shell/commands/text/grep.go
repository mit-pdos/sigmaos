package text

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"sigmaos/shell/shellctx"
	"sigmaos/shell/util"
	"strings"
)

type GrepCommand struct{}

func NewGrepCommand() *GrepCommand {
	return &GrepCommand{}
}

func (c *GrepCommand) Name() string {
	return "grep"
}

func (c *GrepCommand) Usage() string {
	return "grep <pattern> [file]"
}

func (c *GrepCommand) Execute(ctx *shellctx.ShellContext, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) bool {
	if len(args) < 1 || len(args) > 2 {
		fmt.Fprintf(stderr, "Invalid number of arguments\n%s\n", c.Usage())
		return false
	}

	if args[0] == "--help" {
		fmt.Fprintln(stdout, c.Usage())
		fmt.Fprintln(stdout, "\nSearches for lines matching the specified pattern.")
		fmt.Fprintln(stdout, "If no file is provided, grep reads from standard input.")
		return true
	}

	pattern := args[0]
	regex, err := regexp.Compile(pattern)
	if err != nil {
		fmt.Fprintf(stderr, "Invalid regular expression: %v\n", err)
		return false
	}

	var reader io.Reader

	if len(args) == 2 {
		// Read from file
		filename := args[1]
		resolvedPath := util.ResolvePath(ctx, filename)
		data, err := ctx.Tstate.GetFile(shellctx.FILEPATH_OFFSET + resolvedPath)
		if err != nil {
			fmt.Fprintf(stderr, "Error reading file '%s': %v\n", filename, err)
			return false
		}
		reader = strings.NewReader(string(data))
	} else {
		// Read from stdin
		reader = stdin
	}

	scanner := bufio.NewScanner(reader)
	lineNum := 0
	matchFound := false

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		if regex.MatchString(line) {
			matchFound = true
			if len(args) == 2 {
				fmt.Fprintf(stdout, "%s:%d:%s\n", args[1], lineNum, line)
			} else {
				fmt.Fprintf(stdout, "%d:%s\n", lineNum, line)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(stderr, "Error reading input: %v\n", err)
		return false
	}

	if !matchFound {
		fmt.Fprintln(stderr, "No matches found")
		return false
	}

	return true
}
