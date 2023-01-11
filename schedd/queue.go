package schedd

import (
	"encoding/json"

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

func (q *Queue) Enqueue(pstr string) {
	// TODO: make protobuf struct for proc so we don't have to unmarshal here.
	p := proc.MakeEmptyProc()
	if err := json.Unmarshal([]byte(pstr), p); err != nil {
		db.DFatalf("Err unmarshal", err)
	}
	switch p.Type {
	case proc.T_LC:
		q.lc = append(q.lc, p)
	case proc.T_BE:
		q.be = append(q.be, p)
	default:
		db.DFatalf("Unrecognized proc type: %v", p.Type)
	}
}

// LC procs have absolute priority.
func (q *Queue) Dequeue() (string, bool) {
	var p *proc.Proc
	if len(q.lc) > 0 {
		p, q.lc = q.lc[0], q.lc[1:]
	} else if len(q.be) > 0 {
		p, q.be = q.be[0], q.be[1:]
	}
	if p == nil {
		return "", false
	}
	b, err := json.Marshal(p)
	if err != nil {
		db.DFatalf("Error marshal proc: %v", err)
	}
	return string(b), true
}
