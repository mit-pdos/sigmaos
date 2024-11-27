package clnt

import (
	"sync"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

// A pool of booted, but unused, uprocds.
type pool struct {
	sync.Mutex
	cond       *sync.Cond
	startProcd startProcdFn
	clnts      []*ProcClnt
	pids       []sp.Tpid
}

func newPool(fn startProcdFn) *pool {
	p := &pool{
		startProcd: fn,
		clnts:      make([]*ProcClnt, 0, sp.Conf.UProcSrv.POOL_SZ),
		pids:       make([]sp.Tpid, 0, sp.Conf.UProcSrv.POOL_SZ),
	}
	p.cond = sync.NewCond(&p.Mutex)
	return p
}

// Fill the pool.
func (p *pool) fill() {
	p.Lock()
	defer p.Unlock()

	db.DPrintf(db.UPROCDMGR, "Fill uprocd pool len %v target %v", len(p.clnts), sp.Conf.UProcSrv.POOL_SZ)
	for len(p.clnts) < sp.Conf.UProcSrv.POOL_SZ {
		// Unlock to allow clients to take a uprocd off the queue while another is
		// being started
		p.Unlock()
		pid, clnt := p.startProcd()
		// Reclaim lock
		p.Lock()
		p.pids = append(p.pids, pid)
		p.clnts = append(p.clnts, clnt)
		// Wake up any potentially waiting clients
		p.cond.Broadcast()
	}
	db.DPrintf(db.UPROCDMGR, "Done Fill uprocd pool len %v target %v", len(p.clnts), sp.Conf.UProcSrv.POOL_SZ)
	p.cond.Broadcast()
}

func (p *pool) get() (sp.Tpid, *ProcClnt) {
	p.Lock()
	defer p.Unlock()

	// Wait for there to be available uprocds in the pool.
	for len(p.clnts) == 0 {
		db.DPrintf(db.UPROCDMGR, "Wait for uprocd pool to be filled len %v", len(p.clnts))
		db.DPrintf(db.SPAWN_LAT, "Wait for uprocd pool to be filled len %v", len(p.clnts))
		p.cond.Wait()
	}
	db.DPrintf(db.UPROCDMGR, "Pop from uprocd pool")

	var pid sp.Tpid
	var clnt *ProcClnt

	// Pop from the pool of uprocds.
	pid, p.pids = p.pids[0], p.pids[1:]
	clnt, p.clnts = p.clnts[0], p.clnts[1:]

	// Refill asynchronously.
	go p.fill()

	return pid, clnt
}
