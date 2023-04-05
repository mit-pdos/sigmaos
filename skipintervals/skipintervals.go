package skipinterval

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	db "sigmaos/debug"
	"sigmaos/sessp"
)

var MaxLevel = 7

type SkipIntervals struct {
	levels levels
	rand   *rand.Rand
	back   *element
	length int
}

func MkSkipIntervals() *SkipIntervals {
	source := rand.NewSource(time.Now().UnixNano())
	return &SkipIntervals{
		levels: mkLevels(MaxLevel),
		rand:   rand.New(source),
	}
}

func (skipl *SkipIntervals) String() string {
	s := "SkipIntervals:\n"
	for i := MaxLevel - 1; i >= 0; i-- {
		s += fmt.Sprintf("level %d %v\n", i, skipl.levels[i].Level(i))
	}
	return s
}

func (skipl *SkipIntervals) Length() int {
	return skipl.length
}

func (skipl *SkipIntervals) Insert(iv sessp.Tinterval) {
	prevElems := mkLevels(MaxLevel)
	next := skipl.findNext(nil, iv.Start, prevElems)

	db.DPrintf(db.ALWAYS, "Insert %v next %v prevElem %v\n", iv.Marshal(), next, prevElems)
	if next == nil || iv.End < next.iv.Start { // iv preceeds next
		skipl.insert(iv, prevElems, next)
		skipl.merge(prevElems)
	} else { // iv overlaps next
		if iv.End >= next.iv.End {
			next.iv.End = iv.End
		}
		if iv.Start >= next.iv.Start { // iv is in net
			return
		}
		if iv.Start < next.iv.Start {
			next.iv.Start = iv.Start
		}
		skipl.merge(prevElems)
	}
}

func (skipl *SkipIntervals) insert(iv sessp.Tinterval, prevElems levels, next *element) {
	level := skipl.randLevel()
	elem := mkElement(level, iv)

	db.DPrintf(db.ALWAYS, "insert %v %v(%d) %v\n", prevElems, elem, level, skipl)

	// Set previous elements
	elem.prev = prevElems[0]

	// Insert elem at each level
	for i := 0; i < level; i++ {
		if prevElems[i] == nil {
			elem.levels[i] = skipl.levels[i]
			skipl.levels[i] = elem
		} else {
			elem.levels[i] = prevElems[i].levels[i]
			prevElems[i].levels[i] = elem
			db.DPrintf(db.ALWAYS, "insert level %d %s\n", i, prevElems[i].Level(i))

		}
	}

	if next := elem.levels[0]; next == nil {
		skipl.back = elem
	} else {
		next.prev = elem
	}

	db.DPrintf(db.ALWAYS, "inserted %v %v\n", iv.Marshal(), skipl)
	skipl.length++
}

// maybe merge prevelem and elem
func (skipl *SkipIntervals) merge(prevElems levels) {
	if prevElems[0] == nil {
		return
	}
	log.Printf("merge? %v %v\n", prevElems, skipl)
	elem := prevElems[0]
	next := elem.levels[0]
	if elem.iv.End >= next.iv.Start { // merge  elem and next
		if next.iv.End > elem.iv.End {
			elem.iv.End = next.iv.End
		}
		next.iv.Start = elem.iv.Start
		if !elem.iv.Eq(next.iv) {
			panic(fmt.Sprintf("merge: %v %v\n", elem, next))
		}

		// remove next
		for i := 0; i < len(elem.levels); i++ {
			if elem.levels[i] == next {
				if i < len(next.levels) {
					elem.levels[i] = next.levels[i]
				} else {
					elem.levels[i] = nil
				}
			}
		}
		// XXX need to fix up pointers that point to next at level j,
		// where j > elem.levels.
		skipl.length--
		log.Printf("skipl merged %v %v %v\n", elem, next, skipl)
	}
}

func (skipl *SkipIntervals) Delete(iv sessp.Tinterval) {
	prevElems := mkLevels(MaxLevel)
	elem := skipl.findNext(nil, iv.Start, prevElems)
	for elem != nil {

		db.DPrintf(db.TEST, "del: %v elem %v prevElems %v\n", iv.Marshal(), elem, prevElems)

		if iv.End < elem.iv.Start { // iv proceeds elem; done
			break
		}
		if iv.Start < elem.iv.Start { // iv overlaps elem
			iv.Start = elem.iv.Start
		}
		if iv.Start <= elem.iv.Start && iv.End >= elem.iv.End { // delete next
			skipl.del(prevElems, elem)
			// XXX see if more to be deleted
		} else if iv.Start > elem.iv.Start && iv.End >= elem.iv.End {
			elem.iv.End = iv.Start
			// XXX see if next elem  should be deleted?
		} else if elem.iv.Start == iv.Start {
			elem.iv.Start = iv.End
			// XXX see if next elem  should be deleted?
		} else { // split iv
			skipl.insert(*sessp.MkInterval(elem.iv.Start, iv.Start), prevElems, elem)
			elem.iv.Start = iv.End
			break
		}
		db.DPrintf(db.ALWAYS, "skip: %v\n", skipl)
		// need to get next and prevElems for next to see if next should be deleted,
		// and delete it.
		elem = skipl.findNext(nil, iv.Start, prevElems)
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

	if next := elem.levels[0]; next != nil {
		next.prev = elem.prev
	}

	if skipl.back == elem {
		skipl.back = elem.prev
	}

	skipl.length--
}

func (skipl *SkipIntervals) Find(iv sessp.Tinterval) *element {
	return skipl.findNext(nil, iv.Start, nil)
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
		if pe != nil {
			pe[i] = prev
		}
	}
	return elem
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
