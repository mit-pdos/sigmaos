package skipinterval

import (
	"fmt"
	"math/rand"
	"time"

	// db "sigmaos/debug"
	"sigmaos/sessp"
)

const ALLOC = true

var MaxLevel = 7

type SkipIntervals struct {
	levels    levels
	rand      *rand.Rand
	back      *element
	prevElems levels
	length    int
	freelist  *element
}

func MkSkipIInterval() sessp.IIntervals {
	return MkSkipIntervals()
}

func MkSkipIntervals() *SkipIntervals {
	source := rand.NewSource(time.Now().UnixNano())
	skipl := &SkipIntervals{
		levels:    mkLevels(MaxLevel),
		rand:      rand.New(source),
		prevElems: mkLevels(MaxLevel),
	}
	for i := 0; i < 2000; i++ {
		e := mkElement(0, nil)
		skipl.freeElem(e)
	}
	return skipl
}

func (skipl *SkipIntervals) String() string {
	s := "SkipIntervals:\n"
	for i := MaxLevel - 1; i >= 0; i-- {
		s += fmt.Sprintf("level %d %v\n", i, skipl.levels[i].Level(i))
	}
	return s
}

func (skipl *SkipIntervals) allocElem(level int, iv *sessp.Tinterval) *element {
	e := skipl.freelist
	skipl.freelist = e.levels[0]
	for i := 1; i < level; i++ {
		e.levels = append(e.levels, nil)
	}
	e.iv = *iv
	return e
}

func (skipl *SkipIntervals) freeElem(elem *element) {
	elem.topPrev = nil
	elem.prev = nil
	elem.levels = elem.levels[:1]
	elem.levels[0] = skipl.freelist
	skipl.freelist = elem
}

func (skipl *SkipIntervals) Length() int {
	return skipl.length
}

func (skipl *SkipIntervals) Insert(iv *sessp.Tinterval) {
	next := skipl.findNext(nil, iv.Start, skipl.prevElems)

	//db.DPrintf(db.TEST, "Insert %v next %v prevElem %v\n", iv.Marshal(), next, skipl.prevElems)
	if next == nil || iv.End < next.iv.Start { // iv preceeds next
		prev := skipl.prevElems[0]
		if prev != nil && prev.iv.End >= iv.Start { // prev overlaps iv?
			if prev.iv.End < iv.End { // extend prev?
				prev.iv.End = iv.End
				skipl.merge(skipl.prevElems)
			}
			return
		}
		skipl.insert(iv, skipl.prevElems, next)
		skipl.merge(skipl.prevElems)
	} else { // iv overlaps next
		if iv.End >= next.iv.End { // extend next
			next.iv.End = iv.End
		}
		if iv.Start >= next.iv.Start { // iv is inside next
			return
		}
		if iv.Start < next.iv.Start {
			next.iv.Start = iv.Start // prepend to next
		}
		skipl.merge(skipl.prevElems)
	}
}

func (skipl *SkipIntervals) insert(iv *sessp.Tinterval, prevElems levels, next *element) {
	level := skipl.randLevel()
	var elem *element
	if ALLOC {
		elem = skipl.allocElem(level, iv)
	} else {
		elem = mkElement(level, iv)
	}

	//db.DPrintf(db.TEST, "insert %v %v(%d) %v\n", prevElems, elem, level, skipl)

	// Set previous's
	elem.prev = prevElems[0]
	if prev := prevElems[level-1]; prev != nil {
		elem.topPrev = prev
	}

	// Insert elem at each level
	for i := 0; i < level; i++ {
		if prevElems[i] == nil {
			elem.levels[i] = skipl.levels[i]
			skipl.levels[i] = elem
		} else {
			elem.levels[i] = prevElems[i].levels[i]
			prevElems[i].levels[i] = elem
			// db.DPrintf(db.TEST, "insert level %d %s\n", i, prevElems[i].Level(i))

		}
	}

	if next := elem.levels[0]; next == nil {
		skipl.back = elem
	} else {
		next.prev = elem
	}

	for i := 0; i < level; i++ {
		if next := elem.levels[i]; next != nil {
			if len(next.levels) <= level {
				next.topPrev = elem
			}
		}
	}

	// db.DPrintf(db.TEST, "inserted %v %v\n", iv.Marshal(), skipl)
	skipl.length++
}

// maybe merge prevelem and elem
func (skipl *SkipIntervals) merge(prevElems levels) {
	if prevElems[0] == nil {
		return
	}
	elem := prevElems[0]
	next := elem.levels[0]
	if next != nil && elem.iv.End >= next.iv.Start { // merge  elem and next
		if next.iv.End > elem.iv.End {
			elem.iv.End = next.iv.End
		}
		next.iv.Start = elem.iv.Start
		if !elem.iv.Eq(&next.iv) {
			panic(fmt.Sprintf("merge: %v %v\n", elem, next))
		}
		skipl.Prevs(next, prevElems)
		skipl.del(prevElems, next)
	}
}

func (skipl *SkipIntervals) Delete(iv *sessp.Tinterval) {
	elem := skipl.findNext(nil, iv.Start, skipl.prevElems)
	for elem != nil {
		//db.DPrintf(db.TEST, "Delete: %v elem %v prevElems %v\n", iv.Marshal(), elem, skipl.prevElems)
		if iv.End < elem.iv.Start { // iv proceeds elem; done
			break
		}
		if iv.Start < elem.iv.Start { // iv overlaps elem
			iv.Start = elem.iv.Start
		}
		next := elem.levels[0]
		if iv.Start <= elem.iv.Start && iv.End >= elem.iv.End { // delete elem
			skipl.del(skipl.prevElems, elem)
		} else if iv.Start > elem.iv.Start && iv.End >= elem.iv.End {
			elem.iv.End = iv.Start
		} else if elem.iv.Start == iv.Start {
			elem.iv.Start = iv.End
		} else { // split iv
			skipl.insert(sessp.MkInterval(elem.iv.Start, iv.Start), skipl.prevElems, elem)
			elem.iv.Start = iv.End
			break
		}
		//db.DPrintf(db.TEST, "Delete iterate: %v %v %v\n", iv.Marshal(), next, skipl)
		elem = next
		skipl.Prevs(elem, skipl.prevElems)
	}
}

// Remove elem from each level
func (skipl *SkipIntervals) del(prevElems levels, elem *element) {
	for i := 0; i < len(elem.levels); i++ {
		if prevElems[i] == nil {
			skipl.levels[i] = elem.levels[i]
		} else {
			prevElems[i].levels[i] = elem.levels[i]

		}
	}

	// update prevs
	if next := elem.levels[0]; next != nil {
		next.prev = elem.prev
	}
	for i := 0; i < len(elem.levels); {
		next := elem.levels[i]
		if next == nil || next.topPrev != elem {
			break
		}
		i = len(next.levels)
		next.topPrev = prevElems[i-1]
	}

	if skipl.back == elem {
		skipl.back = elem.prev
	}
	if ALLOC {
		skipl.freeElem(elem)
	}
	skipl.length--
}

func (skipl *SkipIntervals) Contains(s uint64) bool {
	if next := skipl.findNext(nil, s, skipl.prevElems); next == nil {
		return false
	} else {
		return s >= next.iv.Start && s < next.iv.End
	}
}

func (skipl *SkipIntervals) Present(iv *sessp.Tinterval) bool {
	if elem := skipl.findNext(nil, iv.Start, skipl.prevElems); elem == nil {
		return false
	} else {
		if iv.End <= elem.iv.Start {
			return false
		}
		if iv.End > elem.iv.End {
			iv.Start = elem.iv.End
			return skipl.Present(iv)
		}
		return true
	}
}

func (skipl *SkipIntervals) Find(iv *sessp.Tinterval) *sessp.Tinterval {
	if elem := skipl.findNext(nil, iv.Start, skipl.prevElems); elem == nil {
		return nil
	} else {
		return &elem.iv
	}
}

// Return first interval whose end is passed start and its predecessors at each level
func (skipl *SkipIntervals) findNext(begin *element, start uint64, pe levels) *element {
	levels := skipl.levels
	if begin != nil {
		levels = begin.levels
	}
	var prev *element
	var elem *element
	for i := MaxLevel - 1; i >= 0; i-- {
		next := levels[i]
		if prev != nil {
			next = prev.levels[i]
		}
		for ; next != nil; next = next.levels[i] {
			if start < next.iv.End {
				elem = next
				break
			}
			prev = next
		}
		pe[i] = prev
	}
	return elem
}

func (skipl *SkipIntervals) Prevs(elem *element, prevElems levels) levels {
	if elem == nil {
		return nil
	}
	prev := elem.topPrev
	for i := len(elem.levels) - 1; i >= 0; i-- {
		if prev == nil {
			prev = skipl.levels[i]
		}
		if prev != elem {
			for next := prev.levels[i]; next != elem; next = next.levels[i] {
				prev = next
			}
			prevElems[i] = prev
		} else {
			prevElems[i] = nil
			prev = nil
		}
	}
	//db.DPrintf(db.TEST, "Prevs: %v(%p) %v in %v\n", elem, elem, prevElems, skipl)
	return prevElems
}

func (skipl *SkipIntervals) randLevel() int {
	const half = 1 << 30
	i := 1
	for ; i < MaxLevel; i++ {
		if skipl.rand.Int31() < half {
			break
		}
	}
	return i
}
