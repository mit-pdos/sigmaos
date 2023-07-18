package leasemgrclnt

import (
	"path"

	db "sigmaos/debug"
	"sigmaos/fsetcd"
	"sigmaos/fslib"
	leaseproto "sigmaos/lease/proto"
	"sigmaos/proc"
	sp "sigmaos/sigmap"
	"sigmaos/syncmap"
)

type LeaseMgrClnt struct {
	*fslib.FsLib
	lm *syncmap.SyncMap[string, *LeaseInfo]
	cc *ClntCache
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
	srv, rest, err := lmc.PathLastSymlink(pn)
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
		return li, nil
	} else {
		db.DPrintf(db.LEASECLNT, "AskLease %v err %v\n", pn, err)
		return nil, err
	}
}

func (lmgr *LeaseMgrClnt) EndLeases() error {
	db.DPrintf(db.LEASECLNT, "%v: EndLeases\n", proc.GetPid())
	for _, li := range lmgr.lm.Values() {
		db.DPrintf(db.LEASECLNT, "%v: EndLeases %v\n", proc.GetPid(), li)
		li.End()
	}
	return nil
}
