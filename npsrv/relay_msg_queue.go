package npsrv

import (
	"sync"

	np "ulambda/ninep"
)

type RelayMsg struct {
	op    *SrvOp
	fcall *np.Fcall
	seqno uint64
}

type RelayMsgQueue struct {
	mu sync.Mutex
	q  []*RelayMsg
}

func (q *RelayMsgQueue) Enqueue(msg *RelayMsg) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.q = append(q.q, msg)
}

// Dequeue all entries up until and including the one labelled as seqno.
// We do this since responses may come back out of order.
func (q *RelayMsgQueue) DequeueUntil(seqno uint64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, m := range q.q {
		// Message with seqno was not present
		if m.seqno > seqno {
			break
		}
		// Trim the front of the queue
		if m.seqno == seqno {
			q.q = q.q[i+1:]
			break
		}
	}
}

func (q *RelayMsgQueue) GetQ() []*RelayMsg {
	q.mu.Lock()
	defer q.mu.Unlock()
	q1 := make([]*RelayMsg, len(q.q))
	copy(q1, q.q)
	return q1
}

// Get the next message following seqno
func (q *RelayMsgQueue) Next(seqno uint64) *RelayMsg {
	q.mu.Lock()
	defer q.mu.Unlock()
	for _, m := range q.q {
		if m.seqno > seqno {
			return m
		}
	}
	return nil
}
