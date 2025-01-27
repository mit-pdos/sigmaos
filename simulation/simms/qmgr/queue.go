package qmgr

import (
	"sort"

	db "sigmaos/debug"
	"sigmaos/simulation/simms"
)

type Queue struct {
	t       *uint64
	items   []*qItem
	sorted  bool
	maxQLen int
}

type qItem struct {
	req   *simms.Request
	qtime uint64 // Time at which the request was enqueued (which may be different than request start time, particularly in the event of retries)
}

func NewQueue(t *uint64, sorted bool, maxQLen int) *Queue {
	return &Queue{
		t:       t,
		items:   []*qItem{},
		sorted:  sorted,
		maxQLen: maxQLen,
	}
}

func (q *Queue) Enqueue(reqs []*simms.Request) {
	for _, r := range reqs {
		q.items = append(q.items, &qItem{
			qtime: *q.t,
			req:   r,
		})
	}
	if q.sorted {
		// If sorted, sort by request start time in ascending order
		sort.Slice(q.items, func(i, j int) bool {
			return q.items[i].req.GetStart() < q.items[j].req.GetStart()
		})
	}
}

func (q *Queue) Dequeue() (*simms.Request, bool) {
	if len(q.items) == 0 {
		return nil, false
	}
	var i *qItem
	i, q.items = q.items[0], q.items[1:]
	return i.req, true
}

// Time out requests which have been queued for longer than timeout.
// If this queue has a max queue length, time out (and retry) requests which
// exceed the max queue lenght.
func (q *Queue) TimeoutReqs(timeout uint64) []*simms.Request {
	// Sanity check
	if timeout > 0 && q.maxQLen > 0 {
		db.DFatalf("Err: queue can't have a timeout *and* a max queue length")
	}
	tos := []*simms.Request{}
	if timeout > 0 {
		for i := 0; i < len(q.items); i++ {
			// If request timed out, retry it
			if *q.t-q.items[i].qtime > timeout {
				// Append to slice of timeouts
				tos = append(tos, q.items[i].req)
				// Remove from queue
				q.items = append(q.items[:i], q.items[i+1:]...)
			}
		}
	} else if q.maxQLen > 0 {
		for len(q.items) > q.maxQLen {
			i := len(q.items) - 1
			// Append to slice of timeouts
			tos = append(tos, q.items[i].req)
			// Remove from queue
			q.items = q.items[:i]
		}
	}
	return tos
}

func (q *Queue) GetLen() int {
	return len(q.items)
}
