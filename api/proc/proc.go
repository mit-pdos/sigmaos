package rooc

import (
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/util/spstats"
)

type ProcAPI interface {
	// Functions for parent proc
	Spawn(p *proc.Proc) error
	Evict(pid sp.Tpid) error
	WaitStart(pid sp.Tpid) error
	WaitExit(pid sp.Tpid) (*proc.Status, error)

	// Functions for child proc
	Started() error
	Exited(status *proc.Status)
	WaitEvict(pid sp.Tpid) error

	Stats() *spstats.TcounterSnapshot
}
