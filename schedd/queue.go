package schedd

import (
	db "sigmaos/debug"
	"sigmaos/proc"
)

type Queue struct {
	lc []*proc.Proc
	be []*proc.Proc
}

func makeQueue() *Queue {
	return &Queue{
		lc: make([]*proc.Proc, 10),
		be: make([]*proc.Proc, 10),
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

// LC procs have absolute priority.
func (q *Queue) Dequeue() (*proc.ProcProto, bool) {
	var p *proc.Proc
	if len(q.lc) > 0 {
		p, q.lc = q.lc[0], q.lc[1:]
	} else if len(q.be) > 0 {
		p, q.be = q.be[0], q.be[1:]
	}
	if p == nil {
		return nil, false
	}
	return p.ProcProto, true
}
