package procqsrv

import (
	"fmt"
	"sync"

	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

const (
	DEF_Q_SZ = 10
)

type Qitem struct {
	p     *proc.Proc
	kidch chan string
}

func newQitem(p *proc.Proc) *Qitem {
	return &Qitem{
		p:     p,
		kidch: make(chan string),
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

// Dequeue a proc with certain resource requirements. LC procs have absolute
// priority.
func (q *Queue) Dequeue() (*proc.Proc, chan string, bool) {
	q.Lock()
	defer q.Unlock()

	var qi *Qitem
	var ok bool
	if len(q.procs) > 0 {
		qi, q.procs = q.procs[0], q.procs[1:]
		ok = true
		delete(q.pmap, qi.p.GetPid())
	}
	return qi.p, qi.kidch, ok
}

func (q *Queue) String() string {
	q.Lock()
	defer q.Unlock()

	return fmt.Sprintf("{ procs:%v }", q.procs)
}
