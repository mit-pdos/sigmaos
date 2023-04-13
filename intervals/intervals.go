package intervals

//
// Package to maintain an ordered list of intervals
//

import (
	"sort"

	// db "sigmaos/debug"
	"sigmaos/sessp"
)

type IIntervals interface {
	Delete(*sessp.Tinterval)
	Insert(*sessp.Tinterval)
	Length() int
}

type IvSlice struct {
	entries []*sessp.Tinterval
}

func MkIInterval() IIntervals {
	return mkIvSlice()
}

func mkIvSlice() *IvSlice {
	return &IvSlice{make([]*sessp.Tinterval, 0)}
}

func (ivs *IvSlice) Length() int {
	return len(ivs.entries)
}

func (ivs *IvSlice) contains(e uint64) bool {
	for _, iv := range ivs.entries {
		if e < iv.Start {
			return false
		}
		if e >= iv.Start && e < iv.End {
			return true
		}
	}
	return false
}

func (ivs *IvSlice) pop() *sessp.Tinterval {
	iv := ivs.entries[0]
	ivs.delidx(0)
	return iv
}

func (ivs *IvSlice) Delete(ivd *sessp.Tinterval) {
	i := ivs.search(ivd.Start)
	for i < len(ivs.entries) {
		iv := ivs.entries[i]
		if ivd.End < iv.Start { // ivd preceeds iv
			return
		}
		// ivd overlaps iv
		if ivd.Start < iv.Start {
			ivd.Start = iv.Start
		}
		if ivd.Start <= iv.Start && ivd.End >= iv.End { // delete i?
			ivs.delidx(i)
		} else if ivd.Start > iv.Start && ivd.End >= iv.End {
			iv.End = ivd.Start
			i++
		} else if ivd.Start == iv.Start {
			iv.Start = ivd.End
			i++
		} else { // split iv
			ivs.insertidx(i, sessp.MkInterval(iv.Start, ivd.Start))
			ivs.entries[i+1].Start = ivd.End
			i += 2
		}
	}
}

// maybe merge with wi with wi+1
func (ivs *IvSlice) merge(i int) {
	iv := ivs.entries[i]
	if len(ivs.entries) > i+1 { // is there a next iv
		iv1 := ivs.entries[i+1]
		if iv.End >= iv1.Start { // merge iv1 into iv
			if iv1.End > iv.End {
				iv.End = iv1.End
			}
			if i+2 == len(ivs.entries) { // trim i+1
				ivs.entries = ivs.entries[:i+1]
			} else {
				ivs.delidx(i + 1)
			}
		}
	}
}

func (ivs *IvSlice) Insert(n *sessp.Tinterval) {
	i := ivs.search(n.Start)
	// If the new entry starts after all of the other entries, append and return.
	if i == len(ivs.entries) {
		ivs.entries = append(ivs.entries, n)
		return
	}

	iv := ivs.entries[i]
	if n.End < iv.Start { // n preceeds iv
		ivs.insertidx(i, n)
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
}

// Delete the ith index of the entries slice.
func (ivs *IvSlice) delidx(i int) {
	copy(ivs.entries[i:], ivs.entries[i+1:])
	ivs.entries = ivs.entries[:len(ivs.entries)-1]
}

// Insert iv at the ith index of the entries slice.
func (ivs *IvSlice) insertidx(i int, iv *sessp.Tinterval) {
	ivs.entries = append(ivs.entries, nil)
	copy(ivs.entries[i+1:], ivs.entries[i:])
	ivs.entries[i] = iv
}

// Search for the index of the first entry for which entry.End is <= start.
func (ivs *IvSlice) search(start uint64) int {
	return sort.Search(len(ivs.entries), func(i int) bool {
		return start <= ivs.entries[i].End
	})
}

func (dst *IvSlice) deepcopy(src *IvSlice) {
	dst.entries = make([]*sessp.Tinterval, len(src.entries))
	for i, iv := range src.entries {
		dst.entries[i] = sessp.MkInterval(iv.Start, iv.End)
	}
}
