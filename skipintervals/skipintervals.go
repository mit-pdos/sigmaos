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

func (skipl *SkipIntervals) Insert(iv sessp.Tinterval) {
	prevElems := mkLevels(MaxLevel)
	elem := skipl.findNext(nil, iv, prevElems)
	log.Printf("%v elem %v prevElems %v\n", iv.Marshal(), elem, prevElems)

	if elem != nil {
		return
	}

	level := skipl.randLevel()
	elem = mkElement(level, iv)

	// Set previous elements
	elem.prev = prevElems[0]
	elem.prevTopLevel = prevElems[level-1]

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

	// Find out the largest level with next element.
	largestLevel := 0
	for i := level - 1; i >= 0; i-- {
		if elem.levels[i] != nil {
			largestLevel = i + 1
			break
		}
	}

	// Adjust prev and prevTopLevel of next elements.
	if next := elem.levels[0]; next != nil {
		next.prev = elem
	}

	for i := 0; i < largestLevel; {
		next := elem.levels[i]
		nextLevel := len(next.levels)

		if nextLevel <= level {
			next.prevTopLevel = elem
		}

		i = nextLevel
	}

	if elem.levels[0] == nil {
		skipl.back = elem
	}

	skipl.length++
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
	for i := MaxLevel - 1; i >= 0; i-- {
		next := levels[i]
		if prev != nil {
			next = prev.levels[i]
		}
		for ; next != nil; next = next.levels[i] {
			if next.iv.Start == iv.Start {
				return next
			}
			if iv.Start < next.iv.Start {
				break
			}
			prev = next
		}
		if pe != nil {
			pe[i] = prev
		}
	}
	return nil
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
