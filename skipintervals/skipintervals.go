package skipinterval

import (
	"fmt"
	"log"
	"math/rand"
	"time"

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
		s1 := fmt.Sprintf("level %d:", i)
		for e := skipl.levels[i]; e != nil; e = e.levels[i] {
			s1 += fmt.Sprintf("%v ", e)
		}
		s += s1 + "\n"
	}
	return s
}

func (skipl *SkipIntervals) Length() int {
	return skipl.length
}

func (skipl *SkipIntervals) Insert(iv sessp.Tinterval) {
	prevElems := mkLevels(MaxLevel)
	next := skipl.findNext(nil, iv, prevElems)

	log.Printf("insert %v next %v prevElem %v\n", iv.Marshal(), next, prevElems)

	if next == nil || iv.End < next.iv.Start {
		skipl.insert(iv, prevElems, next)
		return
	}

	if iv.Start < next.iv.Start { // iv overlaps next
		next.iv.Start = iv.Start
	}

	if iv.End > next.iv.End {
		next.iv.End = iv.End
		skipl.merge(prevElems, next)
		return
	}
}

func (skipl *SkipIntervals) insert(iv sessp.Tinterval, prevElems levels, next *element) {
	level := skipl.randLevel()
	elem := mkElement(level, iv)

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
		}
	}

	if next := elem.levels[0]; next != nil {
		next.prev = elem
	}

	if elem.levels[0] == nil {
		skipl.back = elem
	}

	skipl.length++
}

// maybe merge elem with elem's next
func (skipl *SkipIntervals) merge(prevElems levels, elem *element) {
	if elem.levels[0] == nil {
		return
	}
	log.Printf("skipl %v merge? %v\n", skipl, elem)
	next := elem.levels[0]
	if elem.iv.End >= next.iv.Start { // merge  elem and next
		if next.iv.End > elem.iv.End {
			elem.iv.End = next.iv.End
		}
		next.iv.Start = elem.iv.Start
		if !elem.iv.Eq(next.iv) {
			panic(fmt.Sprintf("merge: %v %v\n", elem, next))
		}
		if len(next.levels) > len(elem.levels) { // delete elem
			for i := 0; i < len(elem.levels); i++ {
				if prevElems[i] == nil {
					skipl.levels[i] = next
				} else {
					prevElems[i].levels[i] = next
				}
			}
		} else { // delete next
			for i := 0; i < len(next.levels); i++ {
				elem.levels[i] = next.levels[i]
			}
		}
		skipl.length--
		log.Printf("skipl %v merged %v %v\n", skipl, elem, next)
	}
}

func (skipl *SkipIntervals) Delete(iv sessp.Tinterval) {
	prevElems := mkLevels(MaxLevel)
	elem := skipl.findNext(nil, iv, prevElems)
	for elem != nil {

		log.Printf("del: %v elem %v prevElems %v\n", iv.Marshal(), elem, prevElems)

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
		log.Printf("skip: %v\n", skipl)
		// need to get next and prevElems for next to see if next should be deleted,
		// and delete it.
		elem = skipl.findNext(nil, iv, prevElems)
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
	return skipl.findNext(nil, iv, nil)
}

func (skipl *SkipIntervals) findNext(start *element, iv sessp.Tinterval, pe levels) *element {
	levels := skipl.levels
	if start != nil {
		levels = start.levels
	}
	var prev *element
	var elem *element
	for i := MaxLevel - 1; i >= 0; i-- {
		next := levels[i]
		if prev != nil {
			next = prev.levels[i]
		}
		for ; next != nil; next = next.levels[i] {
			if iv.Start <= next.iv.End {
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
