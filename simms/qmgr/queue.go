package qmgr

import (
	"sigmaos/simms"
)

type Queue struct {
	reqs []*simms.Request
}

func NewQueue() *Queue {
	return &Queue{
		reqs: []*simms.Request{},
	}
}

func (q *Queue) Enqueue(reqs []*simms.Request) {
	q.reqs = append(q.reqs, reqs...)
}

func (q *Queue) Dequeue() (*simms.Request, bool) {
	if len(q.reqs) == 0 {
		return nil, false
	}
	var req *simms.Request
	req, q.reqs = q.reqs[0], q.reqs[1:]
	return req, true
}

func (q *Queue) GetLen() int {
	return len(q.reqs)
}
