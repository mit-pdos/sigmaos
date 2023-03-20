package queue

import (
	"sync"

	db "sigmaos/debug"
	"sigmaos/sessp"
)

type ReplyQueue struct {
	mu      *sync.Mutex
	cond    *sync.Cond
	replies []*sessp.FcallMsg
	closed  bool
}

func MakeReplyQueue() *ReplyQueue {
	mu := &sync.Mutex{}
	return &ReplyQueue{
		mu:      mu,
		cond:    sync.NewCond(mu),
		replies: make([]*sessp.FcallMsg, 0),
		closed:  false,
	}
}

// Enqueues a reply.
func (q *ReplyQueue) Enqueue(fc *sessp.FcallMsg) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		db.DFatalf("Enqueue on closed replies queue")
	}

	q.replies = append(q.replies, fc)
	q.cond.Signal()
}

// Blocks until replies become available, and dequeues them.
func (q *ReplyQueue) Dequeue() ([]*sessp.FcallMsg, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return nil, false
	}

	for len(q.replies) == 0 {
		q.cond.Wait()
	}

	reps := make([]*sessp.FcallMsg, len(q.replies))
	copy(reps, q.replies)
	q.replies = q.replies[:0]
	return reps, true
}

func (q *ReplyQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.closed = true
}
