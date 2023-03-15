package intervals

//
// Package to maintain an ordered list of intervals
//

import (
	"fmt"
	"sync"

	db "sigmaos/debug"
	"sigmaos/sessp"
)

type Intervals struct {
	sync.Mutex
	entries []*sessp.Tinterval
	next    []*sessp.Tinterval
}

func (ivs *Intervals) String() string {
	return fmt.Sprintf("{ entries:%v next:%v }", ivs.entries, ivs.next)
}

func MkIntervals() *Intervals {
	ivs := &Intervals{}
	ivs.entries = make([]*sessp.Tinterval, 0)
	ivs.next = make([]*sessp.Tinterval, 0)
	return ivs
}

func (ivs *Intervals) First() *sessp.Tinterval {
	ivs.Lock()
	defer ivs.Unlock()

	if len(ivs.entries) == 0 {
		return nil
	}
	return sessp.MkInterval(ivs.entries[0].Start, ivs.entries[0].End)
}

// Spec:
// * Unless ivs.ResetNext is called, the same number should never be returned
// twice from ivs.Next, assuming it was never inserted twice.
// * All intervals inserted in ivs will eventually be returned by Next.
func (ivs *Intervals) Next() *sessp.Tinterval {
	ivs.Lock()
	defer ivs.Unlock()

	if len(ivs.next) == 0 {
		return nil
	}
	var iv *sessp.Tinterval
	// Pop the next interval from the queue.
	iv, ivs.next = ivs.next[0], ivs.next[1:]
	return iv
}

func (ivs *Intervals) ResetNext() {
	ivs.Lock()
	defer ivs.Unlock()

	db.DPrintf(db.INTERVALS, "ResetNext")
	db.DPrintf(db.ALWAYS, "ResetNext")

	// Copy entries to next, to resend all received intervals.
	deepcopy(&ivs.entries, &ivs.next)
}

func (ivs *Intervals) Insert(n *sessp.Tinterval) {
	ivs.Lock()
	defer ivs.Unlock()

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
	for i := 0; i < len(*entries); {
		iv := (*entries)[i]
		if ivd.Start > iv.End { // ivd is beyond iv
			i++
			continue
		}
		if ivd.End < iv.Start { // ivd preceeds iv
			return
		}
		// ivd overlaps iv
		if ivd.Start < iv.Start {
			ivd.Start = iv.Start
		}
		if ivd.Start <= iv.Start && ivd.End >= iv.End { // delete i?
			*entries = append((*entries)[:i], (*entries)[i+1:]...)
		} else if ivd.Start > iv.Start && ivd.End >= iv.End {
			iv.End = ivd.Start
			i++
		} else if ivd.Start == iv.Start {
			iv.Start = ivd.End
			i++
		} else { // split iv
			*entries = append((*entries)[:i+1], (*entries)[i:]...)
			(*entries)[i] = sessp.MkInterval(iv.Start, ivd.Start)
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
				*entries = append((*entries)[:i+1], (*entries)[i+2:]...)
			}
		}
	}
}

func insert(entries *[]*sessp.Tinterval, n *sessp.Tinterval) {
	for i, iv := range *entries {
		if n.Start > iv.End { // n is beyond iv
			continue
		}
		if n.End < iv.Start { // n preceeds iv
			*entries = append((*entries)[:i+1], (*entries)[i:]...)
			(*entries)[i] = n
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
		return
	}
	*entries = append(*entries, n)
}

func deepcopy(src *[]*sessp.Tinterval, dst *[]*sessp.Tinterval) {
	*dst = make([]*sessp.Tinterval, len(*src))
	for i, iv := range *src {
		(*dst)[i] = sessp.MkInterval(iv.Start, iv.End)
	}
}
