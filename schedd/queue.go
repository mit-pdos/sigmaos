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
