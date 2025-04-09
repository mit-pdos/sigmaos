package clnt

import (
	"sync"
	"time"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
	"sigmaos/util/perf"
)

// A pool of booted, but unused, procds.
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

	db.DPrintf(db.PROCDMGR, "Fill procd pool len %v target %v", len(p.clnts), sp.Conf.UProcSrv.POOL_SZ)
	for len(p.clnts) < sp.Conf.UProcSrv.POOL_SZ {
		// Unlock to allow clients to take a procd off the queue while another is
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
	db.DPrintf(db.PROCDMGR, "Done Fill procd pool len %v target %v", len(p.clnts), sp.Conf.UProcSrv.POOL_SZ)
	p.cond.Broadcast()
}

func (p *pool) get() (sp.Tpid, *ProcClnt) {
	p.Lock()
	defer p.Unlock()

	// Wait for there to be available procds in the pool.
	for len(p.clnts) == 0 {
		db.DPrintf(db.PROCDMGR, "Wait for procd pool to be filled len %v", len(p.clnts))
		start := time.Now()
		p.cond.Wait()
		perf.LogSpawnLatency("ProcdMgr.startBalanceShares", sp.NOT_SET, perf.TIME_NOT_SET, start)
	}
	db.DPrintf(db.PROCDMGR, "Pop from procd pool")

	var pid sp.Tpid
	var clnt *ProcClnt

	// Pop from the pool of procds.
	pid, p.pids = p.pids[0], p.pids[1:]
	clnt, p.clnts = p.clnts[0], p.clnts[1:]

	// Refill asynchronously.
	go p.fill()

	return pid, clnt
}
