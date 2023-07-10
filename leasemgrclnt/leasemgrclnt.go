package leasemgrclnt

import (
	"fmt"
	"path"
	"time"

	db "sigmaos/debug"
	"sigmaos/fsetcd"
	"sigmaos/fslib"
	leaseproto "sigmaos/lease/proto"
	"sigmaos/pathclnt"
	"sigmaos/protdevclnt"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/syncmap"
)

type LeaseMgrClnt struct {
	*fslib.FsLib
	lm *syncmap.SyncMap[string, *LeaseInfo]
}

type LeaseInfo struct {
	ch      chan struct{}
	srv     string
	lid     sp.TleaseId
	pdc     *protdevclnt.ProtDevClnt
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
	return &LeaseMgrClnt{FsLib: fsl, lm: syncmap.NewSyncMap[string, *LeaseInfo]()}, nil
}

// Ask for lease; if caller already has a lease at that server, return
// it.
func (lmc *LeaseMgrClnt) AskLease(pn string, ttl sp.Tttl) (*LeaseInfo, error) {
	srv, rest, err := lmc.LastMount(pn, lmc.Uname())
	db.DPrintf(db.LEASEMGRCLNT, "AskLease %v: %v %v err %v\n", pn, srv, rest, err)
	if li, ok := lmc.lm.Lookup(srv.String()); ok {
		return li, nil
	}
	pdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{lmc.FsLib}, path.Join(srv.String(), sp.LEASESRV))
	if err != nil {
		return nil, err
	}
	var res leaseproto.AskResult
	if err := pdc.RPC("LeaseSrv.AskLease", &leaseproto.AskRequest{
		ClntId: uint64(lmc.ClntID()),
		TTL:    fsetcd.LeaseTTL}, &res); err == nil {
		li := &LeaseInfo{
			ch:  make(chan struct{}),
			srv: srv.String(),
			lid: sp.TleaseId(res.LeaseId),
			pdc: pdc, lmc: lmc,
		}
		db.DPrintf(db.LEASEMGRCLNT, "AskLease %q %v\n", srv, li)
		lmc.lm.Insert(srv.String(), li)
		return li, err
	} else {
		return nil, err
	}
}

func (li *LeaseInfo) reconnect() error {
	for i := 0; i < pathclnt.MAXRETRY; i++ {
		time.Sleep(pathclnt.TIMEOUT * time.Millisecond)
		pdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{li.lmc.FsLib}, path.Join(li.srv, sp.LEASESRV))
		if err == nil {
			li.pdc = pdc
			return nil
		}
	}
	return serr.MkErr(serr.TErrUnreachable, li.srv)
}

func (li *LeaseInfo) extendLease() error {
	for {
		var res leaseproto.ExtendResult
		if err := li.pdc.RPC("LeaseSrv.Extend", &leaseproto.ExtendRequest{
			LeaseId: uint64(li.lid),
		}, &res); err != nil {
			break
		} else if serr.IsErrCode(err, serr.TErrUnreachable) {
			if err := li.reconnect(); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
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
				db.DPrintf(db.LEASECLNT, "extender: lid %v err %v\n", li, err)
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
	for {
		var res leaseproto.EndResult
		if err := li.pdc.RPC("LeaseSrv.End", &leaseproto.EndRequest{
			LeaseId: uint64(li.lid),
		}, &res); err != nil {
			break
		} else if serr.IsErrCode(err, serr.TErrUnreachable) {
			if err := li.reconnect(); err != nil {
				return err
			}
		} else {
			return err
		}
	}
	return nil
}
