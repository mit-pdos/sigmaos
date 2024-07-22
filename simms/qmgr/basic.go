package qmgr

import (
	"sigmaos/simms"
)

type BasicQMgr struct {
	q *Queue
}

func NewBasicQMgr() simms.QMgr {
	return &BasicQMgr{
		q: NewQueue(),
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
