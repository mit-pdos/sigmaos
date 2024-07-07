package leaseclnt

import (
	"fmt"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/namesrv/fsetcd"
	leaseproto "sigmaos/lease/proto"
	sp "sigmaos/sigmap"
)

type Lease struct {
	sync.Mutex
	ch      chan struct{}
	srv     string
	lid     sp.TleaseId
	lmc     *LeaseClnt
	expired bool
}

func (l *Lease) Lease() sp.TleaseId {
	return l.lid
}

func (l *Lease) String() string {
	return fmt.Sprintf("{srv %q lid %v}", l.srv, l.lid)
}

func (l *Lease) extendLease() error {
	var res leaseproto.ExtendResult
	db.DPrintf(db.LEASECLNT, "extend lease %v\n", l)
	return l.lmc.cc.RPC(l.srv, "LeaseSrv.Extend", &leaseproto.ExtendRequest{LeaseId: uint64(l.lid)}, &res)
}

func (l *Lease) extender() {
	for {
		select {
		case <-l.ch:
			db.DPrintf(db.LEASECLNT, "extender: end lid %v\n", l)
			return
		case <-time.After(fsetcd.LeaseTTL / 3 * time.Second):
			db.DPrintf(db.LEASECLNT, "extender: extend lid %v\n", l)
			if err := l.extendLease(); err != nil {
				db.DPrintf(db.LEASECLNT, "extender: expire lid %v err %v\n", l, err)

				l.Lock()
				defer l.Unlock()

				// If the lease wasn't ended and already marked as expired, do so.
				if !l.expired {
					// Lease is expired, so no goroutine will be left to wait on the
					// channel. Close it.
					l.expired = true
					close(l.ch)
				}
				return
			}
		}
	}
}

// Extend lease indefinitely
func (l *Lease) KeepExtending() error {
	go l.extender()
	return nil
}

func (l *Lease) End() error {
	db.DPrintf(db.LEASECLNT, "End lid %v", l)
	defer db.DPrintf(db.LEASECLNT, "End lid done %v", l)

	l.Lock()
	defer l.Unlock()

	// If the elase already expired, there is nothing to do. Return immediately.
	if l.expired {
		return nil
	}

	// Lease has expired, so no need to close the channel again.
	l.expired = true

	// Send on, and then close the cahnnel to stop the extender thread
	l.ch <- struct{}{}
	close(l.ch)

	// Tell the server to end the lease.
	var res leaseproto.EndResult
	return l.lmc.cc.RPC(l.srv, "LeaseSrv.End", &leaseproto.EndRequest{LeaseId: uint64(l.lid)}, &res)
}
