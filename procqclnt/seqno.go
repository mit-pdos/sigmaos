package procqclnt

import (
	"sync"

	db "sigmaos/debug"
)

type ProcSeqno struct {
	sync.Mutex
	*sync.Cond
	next uint64
	got  uint64
}

func NewProcSeqno() *ProcSeqno {
	pseqno := &ProcSeqno{
		next: 0,
		got:  0,
	}
	pseqno.Cond = sync.NewCond(&pseqno.Mutex)
	return pseqno
}

// Get the current seqno
func (pc *ProcSeqno) GetNext() uint64 {
	pc.Lock()
	defer pc.Unlock()

	pc.next++
	return pc.next
}

// Set the seqno of the last received proc
func (pc *ProcSeqno) Got(got uint64) {
	pc.Lock()
	defer pc.Unlock()

	// Sanity check. Got should be monotonically increasing
	if got <= pc.got {
		db.DFatalf("Error, got (%v) not monotonically increasing (%v)", pc.got, got)
	}
	pc.got = got
	pc.Broadcast()
}

// Wait until the "got proc" seqno reaches a threshold
func (pc *ProcSeqno) WaitUntilGot(threshold uint64) {
	pc.Lock()
	defer pc.Unlock()

	for threshold > pc.got {
		pc.Wait()
	}
}
