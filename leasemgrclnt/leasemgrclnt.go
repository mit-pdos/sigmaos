package leasemgrclnt

import (
	"fmt"
	"path"
	"sync"
	"time"

	db "sigmaos/debug"
	"sigmaos/fsetcd"
	"sigmaos/fslib"
	leaseproto "sigmaos/lease/proto"
	sp "sigmaos/sigmap"
	"sigmaos/syncmap"
)

type LeaseMgrClnt struct {
	*fslib.FsLib
	lm *syncmap.SyncMap[string, *LeaseInfo]
	cc *ClntCache
}

type LeaseInfo struct {
	sync.Mutex
	ch      chan struct{}
	srv     string
	lid     sp.TleaseId
	lmc     *LeaseMgrClnt
	expired bool
}

func (li *LeaseInfo) Lease() sp.TleaseId {
	return li.lid
}

func (li *LeaseInfo) String() string {
	return fmt.Sprintf("{%q %v}", li.srv, li.lid)
}

func NewLeaseMgrClnt(fsl *fslib.FsLib) (*LeaseMgrClnt, error) {
	return &LeaseMgrClnt{
		FsLib: fsl,
		lm:    syncmap.NewSyncMap[string, *LeaseInfo](),
		cc:    NewClntCache(fsl),
	}, nil
}

// Ask for lease; if caller already has a lease at that server, return
// it.
func (lmc *LeaseMgrClnt) AskLease(pn string, ttl sp.Tttl) (*LeaseInfo, error) {
	srv, rest, err := lmc.LastMount(pn, lmc.Uname())
	db.DPrintf(db.LEASECLNT, "AskLease %v: %v %v err %v\n", pn, srv, rest, err)
	if li, ok := lmc.lm.Lookup(srv.String()); ok {
		return li, nil
	}
	var res leaseproto.AskResult
	if err := lmc.cc.RPC(path.Join(srv.String(), sp.LEASESRV), "LeaseSrv.AskLease", &leaseproto.AskRequest{
		ClntId: uint64(lmc.ClntID()),
		TTL:    fsetcd.LeaseTTL}, &res); err == nil {
		li := &LeaseInfo{
			ch:  make(chan struct{}),
			srv: srv.String(),
			lid: sp.TleaseId(res.LeaseId),
			lmc: lmc,
		}
		db.DPrintf(db.LEASECLNT, "AskLease %q %v\n", srv, li)
		lmc.lm.Insert(srv.String(), li)
		return li, err
	} else {
		return nil, err
	}
}

func (lmgr *LeaseMgrClnt) Exit() error {
	db.DPrintf(db.LEASECLNT, "Exit %v\n", lmgr.lm)
	return nil
}

func (li *LeaseInfo) extendLease() error {
	var res leaseproto.ExtendResult
	return li.lmc.cc.RPC(path.Join(li.srv, sp.LEASESRV), "LeaseSrv.Extend", &leaseproto.ExtendRequest{LeaseId: uint64(li.lid)}, &res)
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
	var res leaseproto.EndResult
	return li.lmc.cc.RPC(path.Join(li.srv, sp.LEASESRV), "LeaseSrv.End", &leaseproto.EndRequest{LeaseId: uint64(li.lid)}, &res)

}
