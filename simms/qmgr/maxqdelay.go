package qmgr

import (
	db "sigmaos/debug"
	"sigmaos/simms"
)

type MaxQDelayQMgr struct {
	q        *Queue
	maxDelay uint64
}

func NewMaxQDelayQMgr(maxDelay uint64) simms.QMgr {
	return &MaxQDelayQMgr{
		q:        NewQueue(),
		maxDelay: maxDelay,
	}
}

func (m *MaxQDelayQMgr) Tick() {
	db.DFatalf("Unimplemented")
}

func (m *MaxQDelayQMgr) Enqueue(req []*simms.Request) {
	m.q.Enqueue(req)
}

func (m *MaxQDelayQMgr) Dequeue() (*simms.Request, bool) {
	return m.q.Dequeue()
}

func (m *MaxQDelayQMgr) GetQLen() int {
	return m.q.GetLen()
}
