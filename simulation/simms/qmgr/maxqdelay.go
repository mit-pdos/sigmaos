package qmgr

import (
	db "sigmaos/debug"
	"sigmaos/simulation/simms"
)

type MaxQDelayQMgr struct {
	t        *uint64
	q        *Queue
	ms       *simms.Microservice
	maxDelay uint64
	sorted   bool
}

func NewMaxQDelayQMgr(t *uint64, maxDelay uint64, sorted bool, ms *simms.Microservice) simms.QMgr {
	return &MaxQDelayQMgr{
		t:        t,
		q:        NewQueue(t, sorted, 0),
		ms:       ms,
		maxDelay: maxDelay,
		sorted:   sorted,
	}
}

func (m *MaxQDelayQMgr) Tick() {
	retries := m.q.TimeoutReqs(m.maxDelay)
	db.DPrintf(db.SIM_QMGR, "Retry timed-out requests %v", retries)
	m.ms.Retry(retries)
}

func (m *MaxQDelayQMgr) Enqueue(req []*simms.Request) {
	m.q.Enqueue(req)
}

func (m *MaxQDelayQMgr) Dequeue() (*simms.Request, bool) {
	return m.q.Dequeue()
}

func (m *MaxQDelayQMgr) GetQ() simms.Queue {
	return m.q
}

func (m *MaxQDelayQMgr) GetQLen() int {
	return m.q.GetLen()
}
