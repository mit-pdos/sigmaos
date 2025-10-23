package main

import (
	"os"

	db "sigmaos/debug"
	"sigmaos/shell"
	"sigmaos/shell/shellctx"
)

func main() {
	if len(os.Args) != 1 {
		db.DFatalf("Usage: %v", os.Args[0])
	}
	ts, err := shellctx.NewTstateAll()
	if err != nil {
		db.DFatalf("error creating tstate: %v", err)
		return
	}
	ctx := shell.NewShell(ts)
	shell.Run(ctx)
}
