package qmgr

import (
	db "sigmaos/debug"
	"sigmaos/simulation/simms"
)

type BasicQMgr struct {
	t  *uint64
	ms *simms.Microservice
	q  *Queue
}

func NewBasicQMgr(t *uint64, ms *simms.Microservice, maxQLen int) simms.QMgr {
	return &BasicQMgr{
		t:  t,
		ms: ms,
		q:  NewQueue(t, false, maxQLen),
	}
}

func (m *BasicQMgr) Tick() {
	retries := m.q.TimeoutReqs(0)
	db.DPrintf(db.SIM_QMGR, "Retry timed-out requests %v", retries)
	m.ms.Retry(retries)
}

func (m *BasicQMgr) Enqueue(req []*simms.Request) {
	m.q.Enqueue(req)
}

func (m *BasicQMgr) Dequeue() (*simms.Request, bool) {
	return m.q.Dequeue()
}

func (m *BasicQMgr) GetQ() simms.Queue {
	return m.q
}

func (m *BasicQMgr) GetQLen() int {
	return m.q.GetLen()
}
