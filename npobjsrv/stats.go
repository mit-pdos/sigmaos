package npobjsrv

import (
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"unsafe"

	np "ulambda/ninep"
)

const STATS = true
const TIMING = false

type Tcounter uint64
type TCycles uint64

func (c *Tcounter) Inc() {
	if STATS {
		n := (*uint64)(unsafe.Pointer(c))
		atomic.AddUint64(n, 1)
	}
}

func (c *TCycles) Add(m uint64) {
	if TIMING {
		n := (*uint64)(unsafe.Pointer(c))
		atomic.AddUint64(n, m)
	}
}

// XXX separate cache lines
type Stats struct {
	Nwalk     Tcounter
	Nclunk    Tcounter
	Nopen     Tcounter
	Nwatchv   Tcounter
	Ncreate   Tcounter
	Nflush    Tcounter
	Nread     Tcounter
	Nreadv    Tcounter
	Nwrite    Tcounter
	Nwritev   Tcounter
	Nremove   Tcounter
	Nstat     Tcounter
	Nwstat    Tcounter
	Nrenameat Tcounter
}

func MkStats() *Stats {
	sts := &Stats{}
	return sts
}

func (st *Stats) Write(off np.Toffset, data []byte) (np.Tsize, error) {
	return 0, nil
}

func (st *Stats) Read(off np.Toffset, n np.Tsize) ([]byte, error) {
	if st == nil {
		return nil, nil
	}
	b := []byte(st.String())
	return b, nil
}

func (st *Stats) Len() np.Tlength {
	if st == nil {
		return 0
	}
	b := []byte(st.String())
	return np.Tlength(len(b))
}

func (st *Stats) String() string {
	v := reflect.ValueOf(*st)
	s := ""
	for i := 0; i < v.NumField(); i++ {
		if i > 0 {
			s += "\n"
		}
		t := v.Field(i).Type().String()
		if strings.HasSuffix(t, "Tcounter") {
			n := v.Field(i).Interface().(Tcounter)
			s += "#" + v.Type().Field(i).Name + ": " + strconv.FormatInt(int64(n), 10)
		}
	}
	return s + "\n"
}
