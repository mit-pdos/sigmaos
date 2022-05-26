package intervals

//
// Package to maintain an ordered list of intervals
//

import (
	"fmt"
	"sync"

	np "ulambda/ninep"
)

type interval struct {
	start np.Toffset
	end   np.Toffset
}

func (iv *interval) String() string {
	return fmt.Sprintf("[%d, %d)", iv.start, iv.end)
}

type Intervals struct {
	sync.Mutex
	ivs []*interval
}

func MkIntervals() *Intervals {
	ivs := &Intervals{}
	ivs.ivs = make([]*interval, 0)
	return ivs
}

// maybe merge with wi with wi+1
func (ivs *Intervals) merge(i int) {
	iv := ivs.ivs[i]
	if len(ivs.ivs) > i+1 { // is there a next iv
		iv1 := ivs.ivs[i+1]
		if iv.end >= iv1.start { // merge iv1 into iv
			if iv1.end > iv.end {
				iv.end = iv1.end
			}
			if i+2 == len(ivs.ivs) { // trim i+1
				ivs.ivs = ivs.ivs[:i+1]
			} else {
				ivs.ivs = append(ivs.ivs[:i+1], ivs.ivs[i+2:]...)
			}
		}
	}
}

func (ivs *Intervals) Insert(n *interval) {
	ivs.Lock()
	defer ivs.Unlock()

	for i, iv := range ivs.ivs {
		if n.start > iv.end { // n is beyond iv
			continue
		}
		if n.end < iv.start { // n preceeds iv
			ivs.ivs = append(ivs.ivs[:i+1], ivs.ivs[i:]...)
			ivs.ivs[i] = n
			return
		}
		// n overlaps iv
		if n.start < iv.start {
			iv.start = n.start
		}
		if n.end > iv.end {
			iv.end = n.end
			ivs.merge(i)
			return
		}
		return
	}
	ivs.ivs = append(ivs.ivs, n)
}

func (ivs *Intervals) Delete(ivd *interval) {
	ivs.Lock()
	defer ivs.Unlock()

	for i := 0; i < len(ivs.ivs); {
		iv := ivs.ivs[i]
		if ivd.start > iv.end { // ivd is beyond iv
			i++
			continue
		}
		if ivd.end < iv.start { // ivd preceeds iv
			return
		}
		// ivd overlaps iv
		if ivd.start < iv.start {
			ivd.start = iv.start
		}
		if ivd.start <= iv.start && ivd.end >= iv.end { // delete i?
			ivs.ivs = append(ivs.ivs[:i], ivs.ivs[i+1:]...)
		} else if ivd.start > iv.start && ivd.end >= iv.end {
			iv.end = ivd.start
			i++
		} else { // split iv
			ivs.ivs = append(ivs.ivs[:i+1], ivs.ivs[i:]...)
			ivs.ivs[i] = &interval{iv.start, ivd.start}
			ivs.ivs[i+1].start = ivd.end
			i += 2
		}
	}
}
