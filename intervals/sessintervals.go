package intervals

//
// Package to maintain intervals for a session
//

import (
	"fmt"
	"sync"

	db "sigmaos/debug"
	"sigmaos/sessp"
	"sigmaos/skipintervals"
	"sigmaos/sliceintervals"
)

const useSkip = false

type Intervals struct {
	sync.Mutex
	sid   sessp.Tsession
	acked sessp.IIntervals // intervals with seqnos for which the server replied
	next  sessp.IIntervals // intervals with seqnos to tell server we have received reply
}

func (ivs *Intervals) String() string {
	return fmt.Sprintf("{ acked:%v next:%v }", ivs.acked, ivs.next)
}

func MkIntervals(sid sessp.Tsession) *Intervals {
	ivs := &Intervals{}
	ivs.sid = sid
	if useSkip {
		ivs.acked = skipintervals.MkSkipIInterval()
		ivs.next = skipintervals.MkSkipIInterval()
	} else {
		ivs.acked = sliceintervals.MkIvSlice()
		ivs.next = sliceintervals.MkIvSlice()
	}
	return ivs
}

// Spec:
// * Unless ivs.ResetNext is called, the same number should never be returned
// twice from ivs.Next, assuming it was never inserted twice.
// * All intervals inserted in ivs will eventually be returned by Next.
func (ivs *Intervals) Next() sessp.Tinterval {
	ivs.Lock()
	defer ivs.Unlock()

	if ivs.next.Length() == 0 {
		db.DPrintf(db.INTERVALS, "[%v] ivs.Next: nil", ivs.sid)
		return sessp.Tinterval{}
	}
	// Pop the next interval from the queue.
	iv := ivs.next.Pop()
	if db.WillBePrinted(db.INTERVALS) {
		db.DPrintf(db.INTERVALS, "[%v] ivs.Next: %v", ivs.sid, iv)
	}
	return iv
}

func (ivs *Intervals) ResetNext() {
	ivs.Lock()
	defer ivs.Unlock()

	db.DPrintf(db.INTERVALS, "[%v] ivs.ResetNext", ivs.sid)

	// Copy acked to next, to resend all received intervals.
	ivs.next.Deepcopy(ivs.acked)

	// db.DPrintf(db.INTERVALS, "ivs.ResetNext next %v acked %v", ivs.next, ivs.acked)
}

func (ivs *Intervals) Insert(n *sessp.Tinterval) {
	ivs.Lock()
	defer ivs.Unlock()

	db.DPrintf(db.INTERVALS, "[%v] ivs.Insert: %v", ivs.sid, n)

	// Insert into next slice, so future calls to ivs.Next will return this
	// interval. Must make a deep copy of n, because it may be modified during
	// insert.
	ivs.next.Insert(sessp.MkInterval(n.Start, n.End))
	// Insert into acked slice.
	ivs.acked.Insert(n)
}

func (ivs *Intervals) Contains(e uint64) bool {
	ivs.Lock()
	defer ivs.Unlock()

	return ivs.acked.Contains(e)
}

func (ivs *Intervals) Delete(ivd *sessp.Tinterval) {
	ivs.Lock()
	defer ivs.Unlock()

	db.DPrintf(db.INTERVALS, "[%v] ivs.Delete: %v", ivs.sid, ivd)

	// Delete from Next slice to ensure the interval isn't returned by ivs.Next.
	ivs.next.Delete(sessp.MkInterval(ivd.Start, ivd.End))
	// Delete from acked slice.
	ivs.acked.Delete(ivd)
}

func (ivs *Intervals) Length() int {
	ivs.Lock()
	defer ivs.Unlock()

	return ivs.acked.Length()
}
