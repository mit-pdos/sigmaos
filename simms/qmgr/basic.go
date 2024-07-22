package qmgr

import (
	"sigmaos/simms"
)

type BasicQMgr struct {
	t *uint64
	q *Queue
}

func NewBasicQMgr(t *uint64) simms.QMgr {
	return &BasicQMgr{
		t: t,
		q: NewQueue(t),
	}
}

func (m *BasicQMgr) Tick() {
	// No-op
}

func (m *BasicQMgr) Enqueue(req []*simms.Request) {
	m.q.Enqueue(req)
}

func (m *BasicQMgr) Dequeue() (*simms.Request, bool) {
	return m.q.Dequeue()
}

func (m *BasicQMgr) GetQLen() int {
	return m.q.GetLen()
}
