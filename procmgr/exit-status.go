package procmgr

import (
	"sigmaos/proc"
)

type ExitStatus struct {
	status []byte
	refcnt int
}

func newExitStatus(p *proc.Proc) *ExitStatus {
	i := 1
	if p.GetType() == proc.T_LC {
		// A hack for GC. Currently, both the parent *and* LCSCHED will WaitExit on
		// LC procs, so we should only GC the exit status if two WaitExits have
		// been received.
		i++
	}
	return &ExitStatus{nil, i}
}

// caller holds lock
func (es *ExitStatus) SetStatus(status []byte) {
	// Only set status once.
	if es.status == nil {
		es.status = status
	}
}

// caller holds lock
func (es *ExitStatus) GetStatus() (status []byte, del bool) {
	es.refcnt--
	return es.status, es.refcnt == 0
}
