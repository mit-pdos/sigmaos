package qmgr

import (
	db "sigmaos/debug"
	"sigmaos/simms"
)

type Queue struct {
	t     *uint64
	qtime []uint64 // Time at which the request was enqueued (which may be different than request start time, particularly in the event of retries)
	reqs  []*simms.Request
}

func NewQueue(t *uint64) *Queue {
	return &Queue{
		t:     t,
		qtime: []uint64{},
		reqs:  []*simms.Request{},
	}
}

func (q *Queue) Enqueue(reqs []*simms.Request) {
	for _ = range reqs {
		q.qtime = append(q.qtime, *q.t)
	}
	q.reqs = append(q.reqs, reqs...)
}

func (q *Queue) Dequeue() (*simms.Request, bool) {
	if len(q.reqs) == 0 {
		return nil, false
	}
	var req *simms.Request
	req, q.reqs = q.reqs[0], q.reqs[1:]
	q.qtime = q.qtime[1:]
	return req, true
}

// Time out requests which have been queued for longer than timeout
func (q *Queue) TimeoutReqs(timeout uint64) []*simms.Request {
	db.DFatalf("Unimplemented")
	return nil
}

func (q *Queue) GetLen() int {
	return len(q.reqs)
}
