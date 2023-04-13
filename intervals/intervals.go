package intervals

//
// Package to maintain an ordered list of intervals
//

import (
	"fmt"
	"sort"
	"sync"

	// db "sigmaos/debug"
	"sigmaos/sessp"
)

type IIntervals interface {
	Delete(*sessp.Tinterval)
	Insert(*sessp.Tinterval)
}

type Intervals struct {
	sync.Mutex
	sid     sessp.Tsession
	entries []*sessp.Tinterval
	next    []*sessp.Tinterval
}

func (ivs *Intervals) String() string {
	return fmt.Sprintf("{ entries:%v next:%v }", ivs.entries, ivs.next)
}

func MkIInterval() IIntervals {
	return MkIntervals(sessp.Tsession(0))
}

func MkIntervals(sid sessp.Tsession) *Intervals {
	ivs := &Intervals{}
	ivs.sid = sid
	ivs.entries = make([]*sessp.Tinterval, 0)
	ivs.next = make([]*sessp.Tinterval, 0)
	return ivs
}

// Spec:
// * Unless ivs.ResetNext is called, the same number should never be returned
// twice from ivs.Next, assuming it was never inserted twice.
// * All intervals inserted in ivs will eventually be returned by Next.
func (ivs *Intervals) Next() *sessp.Tinterval {
	ivs.Lock()
	defer ivs.Unlock()

	if len(ivs.next) == 0 {
		//db.DPrintf(db.INTERVALS, "[%v] ivs.Next: nil", ivs.sid)
		return nil
	}
	// Pop the next interval from the queue.
	iv := ivs.next[0]
	delidx(&ivs.next, 0)
	//if db.WillBePrinted(db.INTERVALS) {
	//db.DPrintf(db.INTERVALS, "[%v] ivs.Next: %v", ivs.sid, iv)
	//}
	return iv
}

func (ivs *Intervals) ResetNext() {
	ivs.Lock()
	defer ivs.Unlock()

	//db.DPrintf(db.INTERVALS, "[%v] ivs.ResetNext", ivs.sid)

	// Copy entries to next, to resend all received intervals.
	deepcopy(&ivs.entries, &ivs.next)
}

func (ivs *Intervals) Insert(n *sessp.Tinterval) {
	ivs.Lock()
	defer ivs.Unlock()

	//db.DPrintf(db.INTERVALS, "[%v] ivs.Insert: %v", ivs.sid, n)

	// Insert into next slice, so future calls to ivs.Next will return this
	// interval. Must make a deep copy of n, because it may be modified during
	// insert.
	insert(&ivs.next, sessp.MkInterval(n.Start, n.End))
	// Insert into entries slice.
	insert(&ivs.entries, n)
}

func (ivs *Intervals) Contains(e uint64) bool {
	ivs.Lock()
	defer ivs.Unlock()

	return contains(ivs.entries, e)
}

func (ivs *Intervals) Delete(ivd *sessp.Tinterval) {
	ivs.Lock()
	defer ivs.Unlock()

	//db.DPrintf(db.INTERVALS, "[%v] ivs.Delete: %v", ivs.sid, ivd)

	// Delete from Next slice to ensure the interval isn't returned by ivs.Next.
	del(&ivs.next, sessp.MkInterval(ivd.Start, ivd.End))
	// Delete from entries slice.
	del(&ivs.entries, ivd)
}

func (ivs *Intervals) Size() int {
	ivs.Lock()
	defer ivs.Unlock()

	return len(ivs.entries)
}

func contains(entries []*sessp.Tinterval, e uint64) bool {
	for _, iv := range entries {
		if e < iv.Start {
			return false
		}
		if e >= iv.Start && e < iv.End {
			return true
		}
	}
	return false
}

func del(entries *[]*sessp.Tinterval, ivd *sessp.Tinterval) {
	i := search(*entries, ivd.Start)
	for i < len(*entries) {
		iv := (*entries)[i]
		if ivd.End < iv.Start { // ivd preceeds iv
			return
		}
		// ivd overlaps iv
		if ivd.Start < iv.Start {
			ivd.Start = iv.Start
		}
		if ivd.Start <= iv.Start && ivd.End >= iv.End { // delete i?
			delidx(entries, i)
		} else if ivd.Start > iv.Start && ivd.End >= iv.End {
			iv.End = ivd.Start
			i++
		} else if ivd.Start == iv.Start {
			iv.Start = ivd.End
			i++
		} else { // split iv
			insertidx(entries, i, sessp.MkInterval(iv.Start, ivd.Start))
			(*entries)[i+1].Start = ivd.End
			i += 2
		}
	}
}

// maybe merge with wi with wi+1
func merge(entries *[]*sessp.Tinterval, i int) {
	iv := (*entries)[i]
	if len(*entries) > i+1 { // is there a next iv
		iv1 := (*entries)[i+1]
		if iv.End >= iv1.Start { // merge iv1 into iv
			if iv1.End > iv.End {
				iv.End = iv1.End
			}
			if i+2 == len(*entries) { // trim i+1
				*entries = (*entries)[:i+1]
			} else {
				delidx(entries, i+1)
			}
		}
	}
}

func insert(entries *[]*sessp.Tinterval, n *sessp.Tinterval) {
	i := search(*entries, n.Start)
	// If the new entry starts after all of the other entries, append and return.
	if i == len(*entries) {
		*entries = append(*entries, n)
		return
	}

	iv := (*entries)[i]
	if n.End < iv.Start { // n preceeds iv
		insertidx(entries, i, n)
		return
	}
	// n overlaps iv
	if n.Start < iv.Start {
		iv.Start = n.Start
	}
	if n.End > iv.End {
		iv.End = n.End
		merge(entries, i)
		return
	}
}

// Delete the ith index of the entries slice.
func delidx(entries *[]*sessp.Tinterval, i int) {
	copy((*entries)[i:], (*entries)[i+1:])
	*entries = (*entries)[:len(*entries)-1]
}

// Insert iv at the ith index of the entries slice.
func insertidx(entries *[]*sessp.Tinterval, i int, iv *sessp.Tinterval) {
	*entries = append(*entries, nil)
	copy((*entries)[i+1:], (*entries)[i:])
	(*entries)[i] = iv
}

// Search for the index of the first entry for which entry.End is <= start.
func search(entries []*sessp.Tinterval, start uint64) int {
	return sort.Search(len(entries), func(i int) bool {
		return start <= entries[i].End
	})
}

func deepcopy(src *[]*sessp.Tinterval, dst *[]*sessp.Tinterval) {
	*dst = make([]*sessp.Tinterval, len(*src))
	for i, iv := range *src {
		(*dst)[i] = sessp.MkInterval(iv.Start, iv.End)
	}
}
