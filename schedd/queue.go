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
	lc []*proc.Proc
	be []*proc.Proc
}

func makeQueue() *Queue {
	return &Queue{
		lc: make([]*proc.Proc, 0, DEF_Q_SZ),
		be: make([]*proc.Proc, 0, DEF_Q_SZ),
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
func (q *Queue) Dequeue(maxcores proc.Tcore, maxmem proc.Tmem) (*proc.Proc, bool) {
	var p *proc.Proc
	// Iterate through LC procs first, checking for core and memory requirements.
	for i := 0; i < len(q.lc); i++ {
		p = q.lc[i]
		// If there are sufficient resources for the LC proc, dequeue it.
		if p.GetNcore() <= maxcores && p.GetMem() <= maxmem {
			q.lc = append(q.lc[:i], q.lc[i+1:]...)
			return p, true
		}
	}
	// Iterate through BE procs second, only checking for memory requirements.
	for i := 0; i < len(q.be); i++ {
		p = q.be[i]
		// If there is sufficient memory for the LC proc, dequeue it.
		if p.GetMem() <= maxmem {
			q.be = append(q.be[:i], q.be[i+1:]...)
			return p, true
		}
	}
	return nil, false
}

func (q *Queue) String() string {
	return fmt.Sprintf("{ lc:%v be:%v }", q.lc, q.be)
}
