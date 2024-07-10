package simms

type Queue struct {
	reqs []*Request
}

func NewQueue() *Queue {
	return &Queue{
		reqs: []*Request{},
	}
}

func (q *Queue) Enqueue(reqs []*Request) {
	q.reqs = append(q.reqs, reqs...)
}

func (q *Queue) Dequeue() (*Request, bool) {
	if len(q.reqs) == 0 {
		return nil, false
	}
	var req *Request
	req, q.reqs = q.reqs[0], q.reqs[1:]
	return req, true
}

func (q *Queue) GetLen() int {
	return len(q.reqs)
}
