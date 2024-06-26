package procqclnt

import (
	"fmt"
	"sync"

	db "sigmaos/debug"
	"sigmaos/proc"
)

type ProcqSession struct {
	sync.Mutex
	*sync.Cond
	procqID string
	epoch   uint64
	next    uint64
	got     uint64
}

func NewProcqSession(procqID string) *ProcqSession {
	pseqno := &ProcqSession{
		procqID: procqID,
		epoch:   1,
		next:    1,
		got:     1,
	}
	pseqno.Cond = sync.NewCond(&pseqno.Mutex)
	return pseqno
}

// Get the current seqno
func (pc *ProcqSession) NextSeqno(scheddID string) *proc.ProcSeqno {
	pc.Lock()
	defer pc.Unlock()

	pc.next++
	return proc.NewProcSeqno(pc.procqID, scheddID, pc.epoch, pc.next)
}

func (pc *ProcqSession) AdvanceEpoch() {
	pc.Lock()
	defer pc.Unlock()

	// Advance epoch
	pc.epoch++
	db.DPrintf(db.SCHEDD, "AdvanceEpoch(%v) sess with procq %v", pc.epoch, pc.procqID)
	// Reset seqnos
	pc.next = 0
	pc.got = 0
	pc.Broadcast()
}

// Set the seqno of the last received proc
func (pc *ProcqSession) Got(pseqno *proc.ProcSeqno) {
	pc.Lock()
	defer pc.Unlock()

	db.DPrintf(db.SCHEDD, "ProcqSession got %v", pseqno)

	// Check if there was a change of epoch
	if pseqno.GetEpoch() > pc.epoch {
		pc.epoch = pseqno.GetEpoch()
		// On epoch change, reset "got" counter
		pc.got = 0
	}
	// Sanity check. Got should be monotonically increasing (unless epoch
	// changed, a case which should be handled above)
	if pseqno.GetSeqno() <= pc.got {
		db.DFatalf("Error, got (%v) not monotonically increasing (%v)", pc.got, pseqno)
	}
	pc.got = pseqno.GetSeqno()
	pc.Broadcast()
}

// Wait until the "got proc" seqno reaches a threshold
func (pc *ProcqSession) WaitUntilGot(pseqno *proc.ProcSeqno) error {
	pc.Lock()
	defer pc.Unlock()

	db.DPrintf(db.SCHEDD, "WaitUntilGot %v", pseqno)
	defer db.DPrintf(db.SCHEDD, "WaitUntilGot done %v", pseqno)

	// Wait while the epoch has not changed, and the awaited sequence number has
	// not been received
	for pseqno.GetEpoch() == pc.epoch && pseqno.GetSeqno() > pc.got {
		pc.Wait()
	}
	// If this proc was sent during a prior epoch of this session, anything might
	// have happened (the proc may have been lost, or it may have run
	// successfully). Return an error to the caller
	if pseqno.GetEpoch() < pc.epoch {
		return fmt.Errorf("Error WaitUntilGot: old epoch (%v < %v)", pseqno.GetEpoch(), pc.epoch)
	}
	return nil
}
