package leaseclnt

import (
	"fmt"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/fsetcd"
	leaseproto "sigmaos/lease/proto"
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
	return fmt.Sprintf("{srv %q lid %v}", li.srv, li.lid)
}

func (li *LeaseInfo) extendLease() error {
	var res leaseproto.ExtendResult
	db.DPrintf(db.LEASECLNT, "extend lease %v\n", li)
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

				li.Lock()
				defer li.Unlock()

				// If the lease wasn't ended and already marked as expired, do so.
				if !li.expired {
					// Lease is expired, so no goroutine will be left to wait on the
					// channel. Close it.
					li.expired = true
					close(li.ch)
				}
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
	db.DPrintf(db.LEASECLNT, "End lid %v", li)
	defer db.DPrintf(db.LEASECLNT, "End lid done %v", li)

	li.Lock()
	defer li.Unlock()

	// If the elase already expired, there is nothing to do. Return immediately.
	if li.expired {
		return nil
	}

	// Lease has expired, so no need to close the channel again.
	li.expired = true

	// Send on, and then close the cahnnel to stop the extender thread
	li.ch <- struct{}{}
	close(li.ch)

	// Tell the server to end the lease.
	var res leaseproto.EndResult
	return li.lmc.cc.RPC(li.srv, "LeaseSrv.End", &leaseproto.EndRequest{LeaseId: uint64(li.lid)}, &res)
}
