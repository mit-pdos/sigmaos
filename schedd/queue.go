package schedd

import (
	"fmt"
	"sync"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

const (
	DEF_Q_SZ = 10
)

type Queue struct {
	sync.Mutex
	lc   []*proc.Proc
	be   []*proc.Proc
	pmap map[sp.Tpid]*proc.Proc
}

func newQueue() *Queue {
	return &Queue{
		lc:   make([]*proc.Proc, 0, DEF_Q_SZ),
		be:   make([]*proc.Proc, 0, DEF_Q_SZ),
		pmap: make(map[sp.Tpid]*proc.Proc, 0),
	}
}

func (q *Queue) Enqueue(p *proc.Proc) {
	q.Lock()
	defer q.Unlock()

	q.pmap[p.GetPid()] = p
	switch p.GetType() {
	case proc.T_LC:
		q.lc = append(q.lc, p)
	case proc.T_BE:
		q.be = append(q.be, p)
	default:
		db.DFatalf("Unrecognized proc type: %v", p.GetType())
	}
}

// Dequeue a proc with certain resource requirements. LC procs have absolute
// priority.
func (q *Queue) Dequeue(ptype proc.Ttype, maxmcpu proc.Tmcpu, maxmem proc.Tmem) (p *proc.Proc, ok bool) {
	q.Lock()
	defer q.Unlock()

	// Get queues holding procs of type ptype.
	qs := q.getQs(ptype)
	for _, queue := range qs {
		if p, ok := dequeue(maxmcpu, maxmem, queue); ok {
			delete(q.pmap, p.GetPid())
			return p, true
		}
	}
	return nil, false
}

// Remove the first proc that fits the maxmcpu & maxmem resource constraints,
// and return it.
func dequeue(maxmcpu proc.Tmcpu, maxmem proc.Tmem, q *[]*proc.Proc) (*proc.Proc, bool) {
	for i := 0; i < len(*q); i++ {
		p := (*q)[i]
		// Sanity check
		if p.GetType() == proc.T_BE && p.GetMcpu() > 0 {
			db.DFatalf("BE proc with mcpu > 0")
		}
		// If there are sufficient resources for the proc, dequeue it. This just
		// involves checking if there are enough mcpu & memory to run it.
		if p.GetMcpu() <= maxmcpu && p.GetMem() <= maxmem {
			*q = append((*q)[:i], (*q)[i+1:]...)
			return p, true
		}
	}
	return nil, false
}

func (q *Queue) getQs(ptype proc.Ttype) []*[]*proc.Proc {
	var qs []*[]*proc.Proc
	switch ptype {
	case proc.T_LC:
		qs = []*[]*proc.Proc{&q.lc}
	case proc.T_BE:
		qs = []*[]*proc.Proc{&q.be}
	default:
		db.DFatalf("Unrecognized proc type: %v", ptype)
	}
	return qs
}

func (q *Queue) String() string {
	q.Lock()
	defer q.Unlock()

	return fmt.Sprintf("{ lc:%v be:%v }", q.lc, q.be)
}
