package interval

import (
	"fmt"
	"log"
	"runtime/debug"
	"strconv"
	"strings"

	sp "sigmaos/sigmap"
)

type Tinterval struct {
	Start uint64
	End   uint64
}

func MkInterval(start, end uint64) *Tinterval {
	return &Tinterval{
		Start: start,
		End:   end,
	}
}

func (iv0 Tinterval) Eq(iv1 *Tinterval) bool {
	return iv0.Start == iv1.Start && iv0.End == iv1.End
}

func (iv *Tinterval) Size() sp.Tsize {
	return sp.Tsize(iv.End - iv.Start)
}

// XXX should atoi be uint64?
func (iv *Tinterval) Unmarshal(s string) {
	idxs := strings.Split(s[1:len(s)-1], ", ")
	start, err := strconv.Atoi(idxs[0])
	if err != nil {
		debug.PrintStack()
		log.Fatalf("FATAL unmarshal interval: %v", err)
	}
	iv.Start = uint64(start)
	end, err := strconv.Atoi(idxs[1])
	if err != nil {
		debug.PrintStack()
		log.Fatalf("FATAL unmarshal interval: %v", err)
	}
	iv.End = uint64(end)
}

func (iv *Tinterval) Marshal() string {
	return fmt.Sprintf("[%d, %d)", iv.Start, iv.End)
}

type IIntervals interface {
	String() string
	Delete(*Tinterval)
	Insert(*Tinterval)
	Length() int
	Contains(uint64) bool
	Present(*Tinterval) bool
	Find(*Tinterval) *Tinterval
	Pop() Tinterval
	Deepcopy(IIntervals)
}
