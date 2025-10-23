package file

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sigmaos/shell/shellctx"
	"sigmaos/shell/util"
	sp "sigmaos/sigmap"
	"strings"
)

type ScpCommand struct{}

func NewScpCommand() *ScpCommand {
	return &ScpCommand{}
}

func (c *ScpCommand) Name() string {
	return "scp"
}

func (c *ScpCommand) Usage() string {
	return "scp <source_file> <destination_file> (local files should start with local://)"
}

func (c *ScpCommand) Execute(ctx *shellctx.ShellContext, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) bool {
	if len(args) != 2 {
		fmt.Fprintf(stderr, "Invalid number of arguments\n%s\n", c.Usage())
		return false
	}
	if args[0] == "--help" {
		fmt.Fprintln(stdout, c.Usage())
		return true
	}

	sourcePath := args[0]
	destPath := args[1]

	// Read source file
	var sourceData []byte
	var err error
	if strings.HasPrefix(sourcePath, "local://") {
		localPath := strings.TrimPrefix(sourcePath, "local://")
		sourceData, err = os.ReadFile(localPath)
	} else {
		resolvedSourcePath := util.ResolvePath(ctx, sourcePath)
		sourceData, err = ctx.Tstate.GetFile(shellctx.FILEPATH_OFFSET + resolvedSourcePath)
	}
	if err != nil {
		fmt.Fprintf(stderr, "Error reading source file: %v\n", err)
		return false
	}

	// Write to destination file
	if strings.HasPrefix(destPath, "local://") {
		localDestPath := strings.TrimPrefix(destPath, "local://")
		err = os.MkdirAll(filepath.Dir(localDestPath), 0755)
		if err != nil {
			fmt.Fprintf(stderr, "Error creating destination directory: %v\n", err)
			return false
		}
		err = os.WriteFile(localDestPath, sourceData, 0644)
	} else {
		resolvedDestPath := util.ResolvePath(ctx, destPath)
		_, err = ctx.Tstate.PutFile(shellctx.FILEPATH_OFFSET+resolvedDestPath, 0777, sp.OWRITE, sourceData)
	}
	if err != nil {
		fmt.Fprintf(stderr, "Error writing destination file: %v\n", err)
		return false
	}

	fmt.Fprintf(stdout, "Successfully copied %s to %s\n", sourcePath, destPath)
	return true
}
