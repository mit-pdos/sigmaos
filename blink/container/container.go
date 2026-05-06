package blink

import (
	"math/rand"

	blinkclnt "sigmaos/blink/clnt"
	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/util/linux/mem"
)

// BlinkClnt wraps the RPC client to BlinkSrv and is held by ProcSrv for
// the lifetime of the server. StartBlinkContainer is a method so the caller
// does not need to pass the client explicitly.
type BlinkClnt struct {
	clnt     *blinkclnt.BlinkClnt
	kernelID string
}

func NewBlinkClnt(kernelID string) (*BlinkClnt, error) {
	c, err := blinkclnt.NewBlinkClnt()
	if err != nil {
		return nil, err
	}
	return &BlinkClnt{clnt: c, kernelID: kernelID}, nil
}

// StartBlinkContainer dispatches a RunBlinkProc RPC to BlinkSrv and returns a
// BlinkContainer whose Wait blocks until the RPC returns (i.e. the proc exits).
func (bc *BlinkClnt) StartBlinkContainer(uproc *proc.Proc) (*BlinkContainer, error) {
	waitC := make(chan error, 1)
	go func() {
		waitC <- bc.clnt.RunBlinkProc(uproc, bc.kernelID)
	}()
	db.DPrintf(db.BLINKD, "StartBlinkContainer pid %v", uproc.GetPid())
	// TODO: replace rand.Int() with PID communicated by BlinkSrv
	return &BlinkContainer{pid: rand.Int(), waitC: waitC}, nil
}

// BlinkContainer implements container.ProcContainer for Blink procs.
type BlinkContainer struct {
	pid   int
	waitC chan error
}

func (bc *BlinkContainer) Pid() int {
	return bc.pid
}

func (bc *BlinkContainer) GetPSS() (proc.Tmem, error) {
	return mem.GetAggregatePSS(bc.pid)
}

func (bc *BlinkContainer) Wait() error {
	return <-bc.waitC
}

// CleanupBlinkProc is a placeholder; Blink manages its own cleanup.
func CleanupBlinkProc(_ sp.Tpid) {}
