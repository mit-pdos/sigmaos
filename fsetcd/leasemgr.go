package fsetcd

import (
	"context"
	"sync"

	"go.etcd.io/etcd/client/v3"

	db "sigmaos/debug"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type leaseMgr struct {
	sync.Mutex
	ec *EtcdClnt
	lc clientv3.Lease
	lt *leaseTable
}

func mkLeaseMgr(ec *EtcdClnt) *leaseMgr {
	return &leaseMgr{ec: ec, lc: clientv3.NewLease(ec.Client), lt: mkLeaseTable()}
}

func (lmgr *leaseMgr) getLeaseID(cid sp.TclntId) (clientv3.LeaseID, error) {
	lmgr.Lock()
	defer lmgr.Unlock()

	lid := lmgr.lt.lookup(cid)
	if lid == clientv3.NoLease {
		resp, err := lmgr.lc.Grant(context.TODO(), SessionTTL)
		if err != nil {
			return clientv3.NoLease, err
		}
		lmgr.keepAlive(cid, resp.ID)
	}
	return lid, nil
}

func (lmgr *leaseMgr) keepAlive(cid sp.TclntId, lid clientv3.LeaseID) error {
	db.DPrintf(db.NAMEDLEASE, "keepAlive cid %v lid %x\n", cid, lid)
	lmgr.lt.add(cid, lid)
	ch, err := lmgr.lc.KeepAlive(context.TODO(), lid)
	if err != nil {
		return err
	}
	go func() {
		for respa := range ch {
			db.DPrintf(db.NAMEDLEASE, "%v %x respa %v\n", cid, lid, respa.TTL)
		}
	}()
	return nil
}

func (lmgr *leaseMgr) recoverLeases(cid sp.TclntId) error {
	lmgr.Lock()
	defer lmgr.Unlock()

	if lid := lmgr.lt.lookup(cid); lid != clientv3.NoLease {
		return nil
	}

	respl, err := lmgr.lc.Leases(context.TODO())
	if err != nil {
		return err
	}
	db.DPrintf(db.NAMEDLEASE, "recoverLeases cid %v %v\n", cid, respl.Leases)
	lopts := make([]clientv3.LeaseOption, 0)
	lopts = append(lopts, clientv3.WithAttachedKeys())
	for _, ls := range respl.Leases {
		respttl, err := lmgr.lc.TimeToLive(context.TODO(), ls.ID, lopts...)
		if err != nil {
			db.DPrintf(db.NAMEDLEASE, "respttl %v err %v\n", ls.ID, err)
			continue
		}
		for _, k := range respttl.Keys {
			db.DPrintf(db.NAMEDLEASE, "respttl %v %v %v\n", cid, respttl.TTL, string(k))
			nf, _, err := lmgr.ec.getFile(string(k))
			if err != nil {
				continue
			}
			db.DPrintf(db.NAMEDLEASE, "getFile %v %v\n", string(k), nf)
			if nf.TclntId() == cid {
				return lmgr.keepAlive(nf.TclntId(), ls.ID)
			}
		}
	}
	return nil
}

func (lmgr *leaseMgr) detach(cid sp.TclntId) {
	lmgr.Lock()
	defer lmgr.Unlock()

	lid := lmgr.lt.lookup(cid)
	db.DPrintf(db.NAMEDLEASE, "named detach %v; revoke %x\n", cid, lid)
	if lid != clientv3.NoLease {
		lmgr.lc.Revoke(context.TODO(), lid)
	}
}

func (lmgr *leaseMgr) LeaseOpts(nf *NamedFile) ([]clientv3.OpOption, *serr.Err) {
	opts := make([]clientv3.OpOption, 0)
	cid := nf.TclntId()
	if cid != sp.NoClntId {
		lid, err := lmgr.getLeaseID(cid)
		if err != nil {
			db.DPrintf(db.ETCDCLNT, "getLeaseID %v err %v\n", cid, err)
			return nil, serr.MkErrError(err)
		}
		opts = append(opts, clientv3.WithLease(lid))
		nf.SetLeaseId(lid)
	}
	return opts, nil
}
