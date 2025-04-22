package shellctx

import "io"

const (
	FILEPATH_OFFSET   = "name"
	LOCAL_FILE_PREFIX = "local://"
)

type Command interface {
	Execute(ctx *ShellContext, args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) bool
	Name() string
	Usage() string
}

// ShellContext represents the shared state for the shell.
type ShellContext struct {
	CurrentDir string
	Tstate     *Tstate
	Commands   map[string]Command
	History    []string
}

func NewShellContext(tstate *Tstate) (*ShellContext, error) {
	return &ShellContext{
		CurrentDir: "/",
		Tstate:     tstate,
		Commands:   make(map[string]Command),
	}, nil
}

func (ctx *ShellContext) RegisterCommands(commands map[string]Command) {
	for name, command := range commands {
		ctx.Commands[name] = command
	}
}

func (ctx *ShellContext) GetCommand(name string) Command {
	command, exists := ctx.Commands[name]
	if !exists {
		return nil
	}
	return command
}
