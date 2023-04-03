package skipinterval

import (
	"fmt"
	"sigmaos/sessp"
)

type levels []*element

func mkLevels(l int) levels {
	return make([]*element, l)
}

type element struct {
	levels       levels
	iv           sessp.Tinterval
	prev         *element
	prevTopLevel *element
}

func mkElement(l int, iv sessp.Tinterval) *element {
	return &element{
		levels: mkLevels(l),
		iv:     iv,
	}
}

func (elem *element) String() string {
	return fmt.Sprintf("%v", elem.iv.Marshal())
}
