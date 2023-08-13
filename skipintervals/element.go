package skipintervals

import (
	"fmt"
	"sigmaos/interval"
)

type levels []*element

func mkLevels(l int) levels {
	return make([]*element, l)
}

func (lv levels) String() string {
	s := "["
	for i := 0; i < len(lv); i++ {
		s += fmt.Sprintf("%p(%v)", lv[i], lv[i])
	}
	return s + "]"
}

type element struct {
	levels  levels
	iv      interval.Tinterval
	topPrev *element
	prev    *element
}

func mkElement(l int, iv *interval.Tinterval) *element {
	e := &element{levels: make([]*element, l, MaxLevel)}
	if iv != nil {
		e.iv = *iv
	}
	return e
}

func (elem *element) String() string {
	s := ""
	if elem.topPrev != nil {
		s = "tp " + elem.topPrev.iv.Marshal()
	}
	return fmt.Sprintf("%v (%s)", elem.iv.Marshal(), s)
}

func (elem *element) Level(l int) string {
	s := ""
	for e := elem; e != nil; e = e.levels[l] {
		s += fmt.Sprintf("%v(%p) ", e, e)
	}
	return s
}
