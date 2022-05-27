package intervals

//
// Package to maintain an ordered list of intervals
//

import (
	"fmt"
	"sync"

	np "ulambda/ninep"
)

type Intervals struct {
	sync.Mutex
	ivs []*np.Tinterval
}

func (ivs *Intervals) String() string {
	return fmt.Sprintf("%v", ivs.ivs)
}

func MkIntervals() *Intervals {
	ivs := &Intervals{}
	ivs.ivs = make([]*np.Tinterval, 0)
	return ivs
}

func (ivs *Intervals) First() *np.Tinterval {
	if len(ivs.ivs) == 0 {
		return nil
	}
	return ivs.ivs[0]
}

func (ivs *Intervals) Len() int {
	return len(ivs.ivs)
}

// maybe merge with wi with wi+1
func (ivs *Intervals) merge(i int) {
	iv := ivs.ivs[i]
	if len(ivs.ivs) > i+1 { // is there a next iv
		iv1 := ivs.ivs[i+1]
		if iv.End >= iv1.Start { // merge iv1 into iv
			if iv1.End > iv.End {
				iv.End = iv1.End
			}
			if i+2 == len(ivs.ivs) { // trim i+1
				ivs.ivs = ivs.ivs[:i+1]
			} else {
				ivs.ivs = append(ivs.ivs[:i+1], ivs.ivs[i+2:]...)
			}
		}
	}
}

func (ivs *Intervals) Insert(n *np.Tinterval) {
	ivs.Lock()
	defer ivs.Unlock()

	for i, iv := range ivs.ivs {
		if n.Start > iv.End { // n is beyond iv
			continue
		}
		if n.End < iv.Start { // n preceeds iv
			ivs.ivs = append(ivs.ivs[:i+1], ivs.ivs[i:]...)
			ivs.ivs[i] = n
			return
		}
		// n overlaps iv
		if n.Start < iv.Start {
			iv.Start = n.Start
		}
		if n.End > iv.End {
			iv.End = n.End
			ivs.merge(i)
			return
		}
		return
	}
	ivs.ivs = append(ivs.ivs, n)
}

// Caller received [start, end), which may increase lower bound of
// what the caller has seen sofar.
func (ivs *Intervals) Prune(lb, start, end np.Toffset) np.Toffset {
	ivs.Insert(np.MkInterval(start, end))
	iv0 := ivs.ivs[0]
	if iv0.Start > lb { // out of order
		return 0
	}
	if iv0.Start < lb { // new data may have straggle off
		iv0.Start = lb
	}
	ivs.ivs = ivs.ivs[1:]
	return iv0.End - iv0.Start
}

func (ivs *Intervals) Delete(ivd *np.Tinterval) {
	ivs.Lock()
	defer ivs.Unlock()

	for i := 0; i < len(ivs.ivs); {
		iv := ivs.ivs[i]
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
			ivs.ivs = append(ivs.ivs[:i], ivs.ivs[i+1:]...)
		} else if ivd.Start > iv.Start && ivd.End >= iv.End {
			iv.End = ivd.Start
			i++
		} else if ivd.Start == iv.Start {
			iv.Start = ivd.End
			i++
		} else { // split iv
			ivs.ivs = append(ivs.ivs[:i+1], ivs.ivs[i:]...)
			ivs.ivs[i] = np.MkInterval(iv.Start, ivd.Start)
			ivs.ivs[i+1].Start = ivd.End
			i += 2
		}
	}
}
