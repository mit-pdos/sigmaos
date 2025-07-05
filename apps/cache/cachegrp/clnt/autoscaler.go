package clnt

import (
	"sync"
	"time"

	"sigmaos/apps/cache/cachegrp/mgr"
	db "sigmaos/debug"
	"sigmaos/rpc"
)

const (
	QLEN_SCALE_THRESHOLD = 2.0
)

type Autoscaler struct {
	sync.Mutex
	cm   *mgr.CacheMgr
	csc  *CachedSvcClnt
	done bool
}

func NewAutoscaler(cm *mgr.CacheMgr, csc *CachedSvcClnt) *Autoscaler {
	return &Autoscaler{
		cm:  cm,
		csc: csc,
	}
}

func (a *Autoscaler) Run(freq time.Duration, max int) {
	go a.run(freq, max)
}

func (a *Autoscaler) Stop() {
	a.Lock()
	defer a.Unlock()
	a.done = true
}

func (a *Autoscaler) AddServers(n int) {
	for i := 0; i < n; i++ {
		a.cm.AddServer()
	}
}

func (a *Autoscaler) run(freq time.Duration, max int) {
	for !a.isDone() {
		sts, err := a.csc.StatsSrvs()
		if err != nil {
			db.DFatalf("Error stats srv: %v", err)
		}
		qlen := globalAvgQlen(sts)
		db.DPrintf(db.ALWAYS, "Global avg cache Qlen: %v", qlen)
		if qlen > QLEN_SCALE_THRESHOLD && len(sts) < max {
			db.DPrintf(db.ALWAYS, "Scale caches up")
			a.AddServers(1)
		}
		time.Sleep(freq)
	}
}

func (a *Autoscaler) isDone() bool {
	a.Lock()
	defer a.Unlock()
	return a.done
}

func globalAvgQlen(sts []*rpc.RPCStatsSnapshot) float64 {
	avg := float64(0.0)
	for i, st := range sts {
		db.DPrintf(db.ALWAYS, "Cache %v qlen: %v", i, st.SrvStatsSnapshot.AvgQlen)
		avg += st.SrvStatsSnapshot.AvgQlen
	}
	return avg / float64(len(sts))
}
