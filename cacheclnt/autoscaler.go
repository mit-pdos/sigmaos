package cacheclnt

import (
	"sync"
	"time"

	"sigmaos/cachedsvc"
	db "sigmaos/debug"
	"sigmaos/rpc"
)

const (
	QLEN_SCALE_THRESHOLD = 2.0
)

type Autoscaler struct {
	sync.Mutex
	cm   *cachedsvc.CacheMgr
	cc   *CacheClnt
	done bool
}

func MakeAutoscaler(cm *cachedsvc.CacheMgr, cc *CacheClnt) *Autoscaler {
	return &Autoscaler{
		cm: cm,
		cc: cc,
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

func (a *Autoscaler) run(freq time.Duration, max int) {
	for !a.isDone() {
		sts, err := a.cc.csc.StatsSrv()
		if err != nil {
			db.DFatalf("Error stats srv: %v", err)
		}
		qlen := globalAvgQlen(sts)
		db.DPrintf(db.ALWAYS, "Global avg cache Qlen: %v", qlen)
		if qlen > QLEN_SCALE_THRESHOLD && len(sts) < max {
			db.DPrintf(db.ALWAYS, "Scale caches up")
			a.cm.AddServer()
		}
		time.Sleep(freq)
	}
}

func (a *Autoscaler) isDone() bool {
	a.Lock()
	defer a.Unlock()
	return a.done
}

func globalAvgQlen(sts []*rpc.SigmaRPCStats) float64 {
	avg := float64(0.0)
	for i, st := range sts {
		db.DPrintf(db.ALWAYS, "Cache %v qlen: %v", i, st.SigmapStat.AvgQlen)
		avg += st.SigmapStat.AvgQlen
	}
	return avg / float64(len(sts))
}
