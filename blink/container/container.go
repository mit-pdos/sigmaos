package blink

import (
	"math/rand"
	"strconv"
	"time"

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
	clnt        *blinkclnt.BlinkClnt
	kernelID    string
	procSrvPID  string
}

func NewBlinkClnt(kernelID string, procSrvPID string) (*BlinkClnt, error) {
	c, err := blinkclnt.NewBlinkClnt()
	if err != nil {
		return nil, err
	}
	return &BlinkClnt{clnt: c, kernelID: kernelID, procSrvPID: procSrvPID}, nil
}

// StartBlinkContainer dispatches a RunBlinkProc RPC to BlinkSrv and returns a
// BlinkContainer whose Wait blocks until the RPC returns (i.e. the proc exits).
func (bc *BlinkClnt) StartBlinkContainer(uproc *proc.Proc) (*BlinkContainer, error) {
	uproc.AppendEnv("PATH", "/bin:/bin2:/usr/bin:/home/sigmaos/bin/kernel")
	uproc.AppendEnv("SIGMA_EXEC_TIME", strconv.FormatInt(time.Now().UnixMicro(), 10))
	b, err := time.Now().MarshalText()
	if err != nil {
		return nil, err
	}
	uproc.AppendEnv("SIGMA_EXEC_TIME_PB", string(b))
	uproc.AppendEnv("SIGMA_SPAWN_TIME", strconv.FormatInt(uproc.GetSpawnTime().UnixMicro(), 10))
	uproc.AppendEnv(proc.SIGMAPERF, uproc.GetProcEnv().GetPerf())
	waitC := make(chan error, 1)
	go func() {
		waitC <- bc.clnt.RunBlinkProc(uproc, bc.kernelID, bc.procSrvPID)
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
