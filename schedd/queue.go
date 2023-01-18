package schedd

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/proc"
)

const (
	DEF_Q_SZ = 10
)

type Queue struct {
	lc   []*proc.Proc
	lcws []*proc.Proc
	be   []*proc.Proc
	bews []*proc.Proc
}

func makeQueue() *Queue {
	return &Queue{
		lc:   make([]*proc.Proc, 0, DEF_Q_SZ),
		lcws: make([]*proc.Proc, 0, DEF_Q_SZ),
		be:   make([]*proc.Proc, 0, DEF_Q_SZ),
		bews: make([]*proc.Proc, 0, DEF_Q_SZ),
	}
}

func (q *Queue) Enqueue(p *proc.Proc) {
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
func (q *Queue) Dequeue(maxcores proc.Tcore, maxmem proc.Tmem) (p *proc.Proc, worksteal bool, ok bool) {
	// Order in which to scan queues.
	qs := []*[]*proc.Proc{&q.lc, &q.lcws, &q.be, &q.bews}
	for i, queue := range qs {
		if p, ok := dequeue(maxcores, maxmem, queue); ok {
			return p, i%2 == 1, true
		}
	}
	return nil, false, false
}

// Remove the first proc that fits the maxcores & maxmem resource constraints,
// and return it.
func dequeue(maxcores proc.Tcore, maxmem proc.Tmem, q *[]*proc.Proc) (*proc.Proc, bool) {
	for i := 0; i < len(*q); i++ {
		p := (*q)[i]
		// Sanity check
		if p.GetType() == proc.T_BE && p.GetNcore() > 0 {
			db.DFatalf("BE proc with ncore > 0")
		}
		// If there are sufficient resources for the LC proc, dequeue it.
		if p.GetNcore() <= maxcores && p.GetMem() <= maxmem {
			*q = append((*q)[:i], (*q)[i+1:]...)
			return p, true
		}
	}
	return nil, false
}

func (q *Queue) String() string {
	return fmt.Sprintf("{ lc:%v be:%v }", q.lc, q.be)
}
