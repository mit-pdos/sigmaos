package procqclnt

import (
	"sync"

	db "sigmaos/debug"
	"sigmaos/proc"
)

type ProcqSession struct {
	sync.Mutex
	*sync.Cond
	epoch uint64
	next  uint64
	got   uint64
}

func NewProcqSession() *ProcqSession {
	pseqno := &ProcqSession{
		epoch: 1,
		next:  1,
		got:   1,
	}
	pseqno.Cond = sync.NewCond(&pseqno.Mutex)
	return pseqno
}

// Get the current seqno
func (pc *ProcqSession) NextSeqno(procqID string, scheddID string) *proc.ProcSeqno {
	pc.Lock()
	defer pc.Unlock()

	pc.next++
	return proc.NewProcSeqno(procqID, scheddID, pc.epoch, pc.next)
}

func (pc *ProcqSession) AdvanceEpoch() {
	pc.Lock()
	defer pc.Unlock()

	// Advance epoch
	pc.epoch++
	// Reset seqnos
	pc.next = 0
	pc.got = 0
	pc.Broadcast()
}

// Set the seqno of the last received proc
func (pc *ProcqSession) Got(pseqno *proc.ProcSeqno) {
	pc.Lock()
	defer pc.Unlock()

	// Sanity check. Got should be monotonically increasing
	if pseqno.GetSeqno() <= pc.got {
		db.DFatalf("Error, got (%v) not monotonically increasing (%v)", pc.got, pseqno)
	}
	pc.got = pseqno.GetSeqno()
	pc.Broadcast()
}

// Wait until the "got proc" seqno reaches a threshold
func (pc *ProcqSession) WaitUntilGot(pseqno *proc.ProcSeqno) {
	pc.Lock()
	defer pc.Unlock()

	for pseqno.GetSeqno() > pc.got {
		pc.Wait()
	}
}
