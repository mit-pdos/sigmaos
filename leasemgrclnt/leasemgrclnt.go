package leasemgrclnt

import (
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
	srv string
	lid sp.TleaseId
	pdc *protdevclnt.ProtDevClnt
	lmc *LeaseMgrClnt
}

func (li *LeaseInfo) Lease() sp.TleaseId {
	return li.lid
}

func NewLeaseMgrClnt(fsl *fslib.FsLib) (*LeaseMgrClnt, error) {
	return &LeaseMgrClnt{FsLib: fsl, lm: syncmap.NewSyncMap[string, *LeaseInfo]()}, nil
}

// Ask for lease; if caller already has a lease at that server, return
// it.
func (lmc *LeaseMgrClnt) AskLease(pn string, ttl sp.Tttl) (*LeaseInfo, error) {
	db.DPrintf(db.LEASEMGRCLNT, "AskLease %v\n", pn)
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
	if err := pdc.RPC("LeaseSrv.AskLease",
		&leaseproto.AskRequest{
			ClntId: uint64(lmc.ClntID()),
			TTL:    fsetcd.LeaseTTL}, &res); err == nil {
		li := &LeaseInfo{srv: srv.String(),
			lid: sp.TleaseId(res.LeaseId),
			pdc: pdc, lmc: lmc,
		}
		lmc.lm.Insert(srv.String(), li)
		return li, err
	} else {
		return nil, err
	}
}

func (li *LeaseInfo) extendLease() error {
	for {
		var res leaseproto.ExtendResult
		if err := li.pdc.RPC("LeaseSrv.Extend",
			&leaseproto.ExtendRequest{
				LeaseId: uint64(li.lid),
			}, &res); err != nil {
			break
		} else if serr.IsErrCode(err, serr.TErrUnreachable) {
			time.Sleep(pathclnt.TIMEOUT * time.Millisecond)
			pdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{li.lmc.FsLib}, path.Join(li.srv, sp.LEASESRV))
			if err != nil {
				continue
			}
			li.pdc = pdc
			continue
		} else {
			return err
		}
	}
	return nil
}

func (lmc *LeaseMgrClnt) Revoke(lid sp.TleaseId) {
}

func (li *LeaseInfo) extender() {
	for {
		time.Sleep(fsetcd.LeaseTTL / 3 * time.Second)
		if err := li.extendLease(); err != nil {
			db.DPrintf(db.LEASECLNT, "extender: lid %v err %v\n", li, err)
		}
	}
}

// Extend lease indefinitely
func (li *LeaseInfo) KeepExtending() error {
	go li.extender()
	return nil
}
