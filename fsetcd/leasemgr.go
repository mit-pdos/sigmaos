package fsetcd

import (
	"context"
	"sync"

	"go.etcd.io/etcd/client/v3"

	db "sigmaos/debug"
	"sigmaos/sessp"
)

const (
	TTL = 10 // seconds
)

type leaseMgr struct {
	sync.Mutex
	lcli clientv3.Lease
	lt   *leaseTable
}

func mkLeaseMgr(cli *clientv3.Client) *leaseMgr {
	return &leaseMgr{lcli: clientv3.NewLease(cli), lt: mkLeaseTable()}
}

func (lmgr *leaseMgr) getLeaseID(sid sessp.Tsession) (clientv3.LeaseID, error) {
	lmgr.Lock()
	defer lmgr.Unlock()

	lid := lmgr.lt.lookup(sid)
	if lid == clientv3.NoLease {
		resp, err := lmgr.lcli.Grant(context.TODO(), TTL)
		if err != nil {
			return clientv3.NoLease, err
		}
		db.DPrintf(db.NAMEDLEASE, "Session %v granted lid %x\n", sid, resp.ID)
		lmgr.lt.add(sid, resp.ID)
		ch, err := lmgr.lcli.KeepAlive(context.TODO(), resp.ID)
		go func() {
			for respa := range ch {
				db.DPrintf(db.NAMEDLEASE, "%v %x respa %v\n", sid, resp.ID, respa.TTL)
			}
		}()
		return resp.ID, nil
	}
	return lid, nil
}

func (lmgr *leaseMgr) detach(sid sessp.Tsession) {
	lmgr.Lock()
	defer lmgr.Unlock()

	lid := lmgr.lt.lookup(sid)
	db.DPrintf(db.NAMEDLEASE, "detach %v; revoke %x\n", sid, lid)
	if lid != clientv3.NoLease {
		lmgr.lcli.Revoke(context.TODO(), lid)
	}
}
