// The leaseclnt package allows clients to obtain leases, which can
// then be attached to files.  If a lease isn't refreshed, the lease
// will expire, and the leased file will be deleted.
package leaseclnt

import (
	db "sigmaos/debug"
	"sigmaos/namesrv/fsetcd"
	"sigmaos/fslib"
	leaseproto "sigmaos/lease/proto"
	"sigmaos/rpcclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmarpcchan"
	"sigmaos/syncmap"
)

type LeaseClnt struct {
	*fslib.FsLib
	lm            *syncmap.SyncMap[string, *Lease]
	cc            *rpcclnt.ClntCache
	askedForLease bool // Used by test harness
}

func NewLeaseClnt(fsl *fslib.FsLib) (*LeaseClnt, error) {
	return &LeaseClnt{
		FsLib: fsl,
		lm:    syncmap.NewSyncMap[string, *Lease](),
		cc:    rpcclnt.NewRPCClntCache(sigmarpcchan.SigmaRPCChanFactory([]*fslib.FsLib{fsl})),
	}, nil
}

// Ask for lease; if caller already has a lease at that server, return
// it.
func (lmc *LeaseClnt) AskLease(pn string, ttl sp.Tttl) (*Lease, error) {
	lmc.askedForLease = true
	srv, rest, err := lmc.PathLastMount(pn)
	db.DPrintf(db.LEASECLNT, "AskLease %v: %v %v err %v\n", pn, srv, rest, err)
	if li, ok := lmc.lm.Lookup(srv.String()); ok {
		return li, nil
	}
	var res leaseproto.AskResult
	if err := lmc.cc.RPC(srv.String(), "LeaseSrv.AskLease", &leaseproto.AskRequest{
		ClntId: uint64(lmc.ClntId()),
		TTL:    fsetcd.LeaseTTL}, &res); err == nil {
		li := &Lease{
			ch:  make(chan struct{}),
			srv: srv.String(),
			lid: sp.TleaseId(res.LeaseId),
			lmc: lmc,
		}
		db.DPrintf(db.LEASECLNT, "AskLease %q lease %v\n", srv, li)
		lmc.lm.Insert(srv.String(), li)
		return li, nil
	} else {
		db.DPrintf(db.LEASECLNT, "AskLease %v err %v\n", pn, err)
		return nil, err
	}
}

func (lmgr *LeaseClnt) EndLeases() error {
	db.DPrintf(db.LEASECLNT, "EndLeases")
	defer db.DPrintf(db.LEASECLNT, "EndLeases done")
	for _, li := range lmgr.lm.Values() {
		db.DPrintf(db.LEASECLNT, "EndLeases %v", li)
		li.End()
		db.DPrintf(db.LEASECLNT, "EndLeases %v done", li)
	}
	return nil
}

func (lmgr *LeaseClnt) AskedForLease() bool {
	return lmgr.askedForLease
}
