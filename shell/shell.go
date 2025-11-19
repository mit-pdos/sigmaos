package shell

import (
	"sigmaos/shell/commands/editor"
	"sigmaos/shell/commands/file"
	"sigmaos/shell/commands/proc"
	"sigmaos/shell/commands/text"
	"sigmaos/shell/shellctx"
)

func NewShell(tstate *shellctx.Tstate) *shellctx.ShellContext {
	ctx, _ := shellctx.NewShellContext(tstate)
	registerCommands(ctx)
	return ctx
}

func registerCommands(ctx *shellctx.ShellContext) {
	// Register Commands
	/// File
	ctx.RegisterCommands(
		map[string]shellctx.Command{
			"cat":   &file.CatCommand{},
			"cd":    &file.CdCommand{},
			"cp":    &file.CpCommand{},
			"ls":    &file.LsCommand{},
			"mkdir": &file.MkdirCommand{},
			"mv":    &file.MvCommand{},
			"pwd":   &file.PwdCommand{},
			"rm":    &file.RmCommand{},
			"touch": &file.TouchCommand{},
			"scp":   &file.ScpCommand{},
		},
	)
	/// Editor
	ctx.RegisterCommands(
		map[string]shellctx.Command{
			"vim":  &editor.VimCommand{},
			"nano": &editor.NanoCommand{},
		},
	)
	/// Proc
	ctx.RegisterCommands(
		map[string]shellctx.Command{
			"evict":     &proc.EvictCommand{},
			"spawn":     &proc.SpawnCommand{},
			"waitstart": &proc.WaitStartCommand{},
			"waitexit":  &proc.WaitExitCommand{},
			"ps":        &proc.PsCommand{},
		},
	)

	/// text
	ctx.RegisterCommands(
		map[string]shellctx.Command{
			"echo": &text.EchoCommand{},
			"grep": &text.GrepCommand{},
			"head": &text.HeadCommand{},
			"tail": &text.TailCommand{},
			"wc":   &text.WcCommand{},
		},
	)
}

func Run(ctx *shellctx.ShellContext) {
	startREPL(ctx)
}
