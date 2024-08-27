package proc

import (
	sp "sigmaos/sigmap"
)

type ProcAPI interface {
	// Functions for parent proc
	Spawn(p *Proc) error
	Evict(pid sp.Tpid) error
	WaitStart(pid sp.Tpid) error
	WaitExit(pid sp.Tpid) (*Status, error)

	// Functions for child proc
	Started() error
	Exited(status *Status)
	WaitEvict(pid sp.Tpid) error
	GetPID() sp.Tpid
	CheckpointMe(pn string) error
}
