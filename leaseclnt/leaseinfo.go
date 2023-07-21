package leaseclnt

import (
	"fmt"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/fsetcd"
	leaseproto "sigmaos/lease/proto"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
)

type LeaseInfo struct {
	sync.Mutex
	ch      chan struct{}
	srv     string
	lid     sp.TleaseId
	lmc     *LeaseClnt
	expired bool
}

func (li *LeaseInfo) Lease() sp.TleaseId {
	return li.lid
}

func (li *LeaseInfo) String() string {
	return fmt.Sprintf("{%q %v}", li.srv, li.lid)
}

func (li *LeaseInfo) extendLease() error {
	var res leaseproto.ExtendResult
	db.DPrintf(db.LEASECLNT, "extend lease %v %v\n", li.srv, li.lid)
	return li.lmc.cc.RPC(li.srv, "LeaseSrv.Extend", &leaseproto.ExtendRequest{LeaseId: uint64(li.lid)}, &res)
}

func (li *LeaseInfo) extender() {
	for {
		select {
		case <-li.ch:
			db.DPrintf(db.LEASECLNT, "extender: end lid %v\n", li)
			return
		case <-time.After(fsetcd.LeaseTTL / 3 * time.Second):
			db.DPrintf(db.LEASECLNT, "extender: extend lid %v\n", li)
			if err := li.extendLease(); err != nil {
				db.DPrintf(db.LEASECLNT, "extender: expire lid %v err %v\n", li, err)
				li.expired = true
				return
			}
		}
	}
}

// Extend lease indefinitely
func (li *LeaseInfo) KeepExtending() error {
	go li.extender()
	return nil
}

func (li *LeaseInfo) End() error {
	li.ch <- struct{}{}
	db.DPrintf(db.LEASECLNT, "%v: End lid %v\n", proc.GetPid(), li)
	var res leaseproto.EndResult
	return li.lmc.cc.RPC(li.srv, "LeaseSrv.End", &leaseproto.EndRequest{LeaseId: uint64(li.lid)}, &res)

}
