package npsrv

import (
	"github.com/sasha-s/go-deadlock"

	"fmt"
	//	"sync"

	np "ulambda/ninep"
)

type RelayMsg struct {
	op    *SrvOp
	fcall *np.Fcall
	seqno uint64
}

type RelayMsgQueue struct {
	mu deadlock.Mutex
	q  []*RelayMsg
}

func (q *RelayMsgQueue) Enqueue(msg *RelayMsg) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.q = append(q.q, msg)
}

// Enqueue another copy of this message if it's already in the queue. Return
// true on success, and false otherwise.
func (q *RelayMsgQueue) EnqueueIfDuplicate(msg *RelayMsg) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	for i, m := range q.q {
		if m.seqno == msg.seqno {
			// Copy the reply
			msg.op.reply = m.op.reply
			q.q = append(append(q.q[:i], msg), q.q[i:]...)
			return true
		}
	}
	return false
}

// Dequeue all entries up until and including the one labelled as seqno.
// We do this since responses may come back out of order.
func (q *RelayMsgQueue) DequeueUntil(seqno uint64) []*RelayMsg {
	q.mu.Lock()
	defer q.mu.Unlock()
	msgs := []*RelayMsg{}
	start := -1
	end := -1
	for i, m := range q.q {
		// Done scanning
		if m.seqno > seqno {
			break
		}
		// Mark where to start & stop trimming, and record message to be trimmed
		if m.seqno <= seqno {
			if start == -1 {
				start = i
			}
			end = i
			msgs = append(msgs, m)
		}
	}
	// Trim the queue
	if len(msgs) > 0 {
		q.q = append(q.q[:start], q.q[end+1:]...)
	}
	return msgs
}

// Dequeue an entry with seqno
func (q *RelayMsgQueue) Dequeue(seqno uint64) (*RelayMsg, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.dequeueL(seqno)
}

func (q *RelayMsgQueue) dequeueL(seqno uint64) (*RelayMsg, bool) {
	for i, m := range q.q {
		if m.seqno == seqno {
			q.q = append(q.q[:i], q.q[i+1:]...)
			return m, true
		}
	}
	return nil, false
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

func (m *RelayMsg) String() string {
	return fmt.Sprintf("{ seqno:%v op:%v fcall:%v }", m.seqno, m.op, m.fcall)
}
