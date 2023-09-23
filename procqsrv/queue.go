package procqsrv

import (
	"fmt"
	"sync"
	"time"

	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

const (
	DEF_Q_SZ = 10
)

type Qitem struct {
	p     *proc.Proc
	kidch chan string
	enqTS time.Time
}

func newQitem(p *proc.Proc) *Qitem {
	return &Qitem{
		p:     p,
		kidch: make(chan string),
		enqTS: time.Now(),
	}
}

type Queue struct {
	sync.Mutex
	procs []*Qitem
	pmap  map[sp.Tpid]*proc.Proc
}

func newQueue() *Queue {
	return &Queue{
		procs: make([]*Qitem, 0, DEF_Q_SZ),
		pmap:  make(map[sp.Tpid]*proc.Proc, 0),
	}
}

func (q *Queue) Enqueue(p *proc.Proc) chan string {
	q.Lock()
	defer q.Unlock()

	q.pmap[p.GetPid()] = p
	qi := newQitem(p)
	q.procs = append(q.procs, qi)
	return qi.kidch
}

func (q *Queue) Dequeue() (*proc.Proc, chan string, time.Time, bool) {
	q.Lock()
	defer q.Unlock()

	var p *proc.Proc
	var ok bool
	var kidch chan string = nil
	var enqTS time.Time
	if len(q.procs) > 0 {
		var qi *Qitem
		qi, q.procs = q.procs[0], q.procs[1:]
		p = qi.p
		kidch = qi.kidch
		enqTS = qi.enqTS
		ok = true
		delete(q.pmap, qi.p.GetPid())
	}
	return p, kidch, enqTS, ok
}

func (q *Queue) String() string {
	q.Lock()
	defer q.Unlock()

	return fmt.Sprintf("{ procs:%v }", q.procs)
}
