package fsetcd

import (
	"sync"

	"go.etcd.io/etcd/client/v3"

	db "sigmaos/debug"
	"sigmaos/sessp"
)

type leaseTable struct {
	sync.Mutex
	leases map[sessp.Tsession]clientv3.LeaseID
}

func mkLeaseTable() *leaseTable {
	lt := &leaseTable{}
	lt.leases = make(map[sessp.Tsession]clientv3.LeaseID)
	return lt
}

func (lt *leaseTable) lookup(sid sessp.Tsession) clientv3.LeaseID {
	lt.Lock()
	defer lt.Unlock()
	lid, ok := lt.leases[sid]
	if ok {
		return lid
	}
	return clientv3.NoLease
}

func (lt *leaseTable) add(sid sessp.Tsession, lid clientv3.LeaseID) {
	lt.Lock()
	defer lt.Unlock()

	_, ok := lt.leases[sid]
	if ok {
		db.DFatalf("add: %v exits\n", sid)
	}
	lt.leases[sid] = lid
}
