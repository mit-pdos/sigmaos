package schedd

import (
	"fmt"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

const (
	DEF_Q_SZ = 10
)

type Queue struct {
	sync.Mutex
	lc   []*proc.Proc
	lcws []*proc.Proc
	be   []*proc.Proc
	bews []*proc.Proc
	pmap map[sp.Tpid]*proc.Proc
}

func makeQueue() *Queue {
	return &Queue{
		lc:   make([]*proc.Proc, 0, DEF_Q_SZ),
		lcws: make([]*proc.Proc, 0, DEF_Q_SZ),
		be:   make([]*proc.Proc, 0, DEF_Q_SZ),
		bews: make([]*proc.Proc, 0, DEF_Q_SZ),
		pmap: make(map[sp.Tpid]*proc.Proc, 0),
	}
}

func (q *Queue) Enqueue(p *proc.Proc) {
	q.Lock()
	defer q.Unlock()

	q.pmap[p.GetPid()] = p
	switch p.GetType() {
	case proc.T_LC:
		q.lc = append(q.lc, p)
	case proc.T_BE:
		q.be = append(q.be, p)
	default:
		db.DFatalf("Unrecognized proc type: %v", p.GetType())
	}
}

// Dequeue a proc with certain resource requirements. LC procs have absolute
// priority.
func (q *Queue) Dequeue(ptype proc.Ttype, maxmcpu proc.Tmcpu, maxmem proc.Tmem) (p *proc.Proc, worksteal bool, ok bool) {
	q.Lock()
	defer q.Unlock()

	// Get queues holding procs of type ptype.
	qs := q.getQs(ptype)
	for i, queue := range qs {
		if p, ok := dequeue(maxmcpu, maxmem, queue); ok {
			worksteal = i%2 == 1
			// If not stolen, remove from pmap
			if !worksteal {
				delete(q.pmap, p.GetPid())
			}
			return p, worksteal, true
		}
	}
	return nil, false, false
}

// Remove a stolen proc from the corresponding queue.
func (q *Queue) Steal(pid sp.Tpid) (*proc.Proc, bool) {
	q.Lock()
	defer q.Unlock()

	// If proc is still queued at this schedd
	if p, ok := q.pmap[pid]; ok {
		// Select queue
		var queue *[]*proc.Proc
		switch p.GetType() {
		case proc.T_LC:
			queue = &q.lc
		case proc.T_BE:
			queue = &q.be
		default:
			db.DFatalf("Unrecognized proc type: %v", p.GetType())
		}
		delete(q.pmap, pid)
		// Scan queue and remove the proc.
		for i, qp := range *queue {
			if qp == p {
				*queue = append((*queue)[:i], (*queue)[i+1:]...)
				break
			}
		}
		return p, true
	}
	return nil, false
}

func (q *Queue) SetWSQueue(qtype proc.Ttype, newQ []*proc.Proc) {
	q.Lock()
	defer q.Unlock()
	switch qtype {
	case proc.T_LC:
		q.lcws = newQ
	case proc.T_BE:
		q.bews = newQ
	default:
		db.DFatalf("Unrecognized queue type: %v", qtype)
	}
}

func (q *Queue) GetStealableProcs(stealable map[sp.Tpid]*proc.Proc) {
	q.Lock()
	defer q.Unlock()

	// Iterate the procs in each realm's queue.
	for _, p := range q.pmap {
		// If this proc has not been spawned for a long time, prepare to offer
		// it as stealable.
		if time.Since(p.GetSpawnTime()) >= sp.Conf.Schedd.STEALABLE_PROC_TIMEOUT {
			stealable[p.GetPid()] = p
		}
	}
}

// Remove the first proc that fits the maxmcpu & maxmem resource constraints,
// and return it.
func dequeue(maxmcpu proc.Tmcpu, maxmem proc.Tmem, q *[]*proc.Proc) (*proc.Proc, bool) {
	for i := 0; i < len(*q); i++ {
		p := (*q)[i]
		// Sanity check
		if p.GetType() == proc.T_BE && p.GetMcpu() > 0 {
			db.DFatalf("BE proc with mcpu > 0")
		}
		// If there are sufficient resources for the proc, dequeue it. This just
		// involves checking if there are enough mcpu & memory to run it.
		if p.GetMcpu() <= maxmcpu && p.GetMem() <= maxmem {
			*q = append((*q)[:i], (*q)[i+1:]...)
			return p, true
		}
	}
	return nil, false
}

func (q *Queue) getQs(ptype proc.Ttype) []*[]*proc.Proc {
	var qs []*[]*proc.Proc
	switch ptype {
	case proc.T_LC:
		qs = []*[]*proc.Proc{&q.lc, &q.lcws}
	case proc.T_BE:
		qs = []*[]*proc.Proc{&q.be, &q.bews}
	default:
		db.DFatalf("Unrecognized proc type: %v", ptype)
	}
	return qs
}

func (q *Queue) String() string {
	q.Lock()
	defer q.Unlock()

	return fmt.Sprintf("{ lc:%v be:%v lcws:%v bews:%v }", q.lc, q.be, q.lcws, q.bews)
}
