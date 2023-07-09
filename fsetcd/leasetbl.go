package fsetcd

import (
	"sync"

	"go.etcd.io/etcd/client/v3"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

type leaseTable struct {
	sync.Mutex
	leases map[sp.TclntId]clientv3.LeaseID
}

func newLeaseTable() *leaseTable {
	lt := &leaseTable{}
	lt.leases = make(map[sp.TclntId]clientv3.LeaseID)
	return lt
}

func (lt *leaseTable) lookup(cid sp.TclntId) clientv3.LeaseID {
	lt.Lock()
	defer lt.Unlock()
	lid, ok := lt.leases[cid]
	if ok {
		return lid
	}
	return clientv3.NoLease
}

func (lt *leaseTable) add(cid sp.TclntId, lid clientv3.LeaseID) {
	lt.Lock()
	defer lt.Unlock()

	_, ok := lt.leases[cid]
	if ok {
		db.DFatalf("add: %v exits\n", cid)
	}
	lt.leases[cid] = lid
}
