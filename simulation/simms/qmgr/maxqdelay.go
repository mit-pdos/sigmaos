package qmgr

import (
	db "sigmaos/debug"
	"sigmaos/simms"
)

type MaxQDelayQMgr struct {
	t        *uint64
	q        *Queue
	ms       *simms.Microservice
	maxDelay uint64
}

func NewMaxQDelayQMgr(t *uint64, maxDelay uint64, ms *simms.Microservice) simms.QMgr {
	return &MaxQDelayQMgr{
		t:        t,
		q:        NewQueue(t),
		ms:       ms,
		maxDelay: maxDelay,
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

func (m *MaxQDelayQMgr) GetQLen() int {
	return m.q.GetLen()
}
