package schedqueue

import (
	"fmt"
	"sync"
	"time"

	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type EligibilityFn func(*proc.Proc) bool

type QItem[T any, C chan T] struct {
	next  *QItem[T, C]
	prev  *QItem[T, C]
	p     *proc.Proc
	ch    C
	enqTS time.Time
}

func newQItem[T any, C chan T](p *proc.Proc, ch C) *QItem[T, C] {
	return &QItem[T, C]{
		next:  nil,
		prev:  nil,
		p:     p,
		ch:    ch,
		enqTS: time.Now(),
	}
}

type Queue[T any, C chan T] struct {
	sync.Mutex
	head *QItem[T, C]
	tail *QItem[T, C]
	pmap map[sp.Tpid]*proc.Proc
}

func NewQueue[T any, C chan T]() *Queue[T, C] {
	return &Queue[T, C]{
		head: nil,
		tail: nil,
		pmap: make(map[sp.Tpid]*proc.Proc, 0),
	}
}

func (q *Queue[T, C]) Enqueue(p *proc.Proc, ch C) {
	q.Lock()
	defer q.Unlock()

	q.pmap[p.GetPid()] = p
	qi := newQItem[T, C](p, ch)
	q.enqueueL(qi)
}

func (q *Queue[T, C]) Dequeue(isEligible EligibilityFn) (*proc.Proc, C, time.Time, bool) {
	q.Lock()
	defer q.Unlock()

	for qi := q.head; qi != nil; qi = qi.next {
		if isEligible(qi.p) {
			q.dequeueL(qi)
			delete(q.pmap, qi.p.GetPid())
			return qi.p, qi.ch, qi.enqTS, true
		}
	}
	return nil, nil, time.UnixMicro(0), false
}

func (q *Queue[T, C]) Len() int {
	q.Lock()
	defer q.Unlock()

	return len(q.pmap)
}

func (q *Queue[T, C]) String() string {
	q.Lock()
	defer q.Unlock()

	return fmt.Sprintf("{ procs:%v }", q.pmap)
}

// Caller must protect returned map with a lock
func (q *Queue[T, C]) GetPMapL() map[sp.Tpid]*proc.Proc {
	return q.pmap
}

func (q *Queue[T, C]) enqueueL(qi *QItem[T, C]) {
	if q.head == nil {
		// If empty, set the new item as the head
		q.head = qi
	} else {
		q.tail.next = qi
		qi.prev = q.tail
	}
	// Set the item as the new tail
	q.tail = qi
}

func (q *Queue[T, C]) dequeueL(qi *QItem[T, C]) {
	next := qi.next
	prev := qi.prev
	if qi != q.head {
		// If this is not the head, set prev's forward-pointer
		prev.next = next
	}
	if qi != q.tail {
		// If this is not the tail, set next's back-pointer
		next.prev = prev
	}
	// If qi was the head, move the head pointer forward
	if qi == q.head {
		q.head = next
	}
	// If qi was the tail, move the tail pointer backward
	if qi == q.tail {
		q.tail = prev
	}
	qi.next = nil
	qi.prev = nil
}
