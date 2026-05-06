package blink

import (
	"math/rand"
	"os/exec"

	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/util/linux/mem"
)

type BlinkContainer struct {
	cmd *exec.Cmd
	pid int // PID of the restored process; stub uses a random value
}

// StartBlinkContainer restores a proc from a Blink snapshot by invoking the
// `blink` CLI tool. Blink manages snapshot storage and binary staging
// transparently; no download or overlay setup is needed here.
// TODO: determine the exact blink command and arguments.
func StartBlinkContainer(uproc *proc.Proc) (*BlinkContainer, error) {
	// TODO: build exec.Command("blink", <args derived from uproc>...)
	// TODO: set cmd.Env, cmd.Stdout, cmd.Stderr
	// TODO: cmd.Start()
	// TODO: replace random PID with the real PID communicated by blink
	return &BlinkContainer{pid: rand.Int()}, nil
}

func (bc *BlinkContainer) Pid() int {
	return bc.pid
}

func (bc *BlinkContainer) GetPSS() (proc.Tmem, error) {
	return mem.GetAggregatePSS(bc.pid)
}

func (bc *BlinkContainer) Wait() error {
	// TODO: bc.cmd.Wait()
	return nil
}

// CleanupBlinkProc is a placeholder; Blink manages its own cleanup.
func CleanupBlinkProc(_ sp.Tpid) {}
