package editor

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"sigmaos/shell/shellctx"
	"sigmaos/shell/util"
	sp "sigmaos/sigmap"
)

type NanoCommand struct{}

func NewNanoCommand() *NanoCommand {
	return &NanoCommand{}
}

func (c *NanoCommand) Name() string {
	return "nano"
}

func (c *NanoCommand) Usage() string {
	return "nano <filename>"
}
func (c *NanoCommand) Execute(ctx *shellctx.ShellContext, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) bool {
	if len(args) != 1 {
		fmt.Fprintf(stderr, "Invalid number of arguments\n %v", c.Usage())
		return false
	}
	if args[0] == "--help" {
		fmt.Fprintln(stdout, c.Usage())
		return true
	}

	filename := args[0]
	fullPath := util.ResolvePath(ctx, filename)

	// Read the file content
	content, err := ctx.Tstate.GetFile(shellctx.FILEPATH_OFFSET + fullPath)
	if err != nil {
		success := ctx.Commands["touch"].Execute(ctx, []string{filename}, stdin, stdout, stderr)
		if !success {
			return false
		}
	}

	// Create a temporary file
	tmpfile, err := ioutil.TempFile("", "edit-*.txt")
	if err != nil {
		fmt.Fprintf(stderr, "error creating temporary file: %v", err)
		return false
	}
	defer os.Remove(tmpfile.Name())

	// Write content to temporary file
	if _, err := tmpfile.Write(content); err != nil {
		fmt.Fprintf(stderr, "error writing to temporary file: %v", err)
		return false
	}
	tmpfile.Close()

	// Start nano process
	cmd := exec.Command("nano", tmpfile.Name())
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(stderr, "error running nano: %v", err)
		return false
	}

	// Read the edited content
	editedContent, err := ioutil.ReadFile(tmpfile.Name())
	if err != nil {
		fmt.Fprintf(stderr, "error reading edited file: %v", err)
		return false

	}

	// Write the edited content back to the original file
	n, err := ctx.Tstate.PutFile(shellctx.FILEPATH_OFFSET+fullPath, 0777, sp.OWRITE, editedContent)
	if err != nil {
		fmt.Fprintf(stderr, "error writing edited content back to file: %v", err)
		return false
	}

	fmt.Fprintf(stdout, "File %s edited successfully, written %d bytes\n", filename, n)
	return true
}
