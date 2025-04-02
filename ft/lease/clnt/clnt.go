// The leaseclnt package allows clients to obtain leases, which can
// then be attached to files.  If a lease isn't refreshed, the lease
// will expire, and the leased file will be deleted.
package clnt

import (
	db "sigmaos/debug"
	leaseproto "sigmaos/ft/lease/proto"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
	"sigmaos/util/syncmap"
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
		cc:    rpcclnt.NewRPCClntCache(sprpcclnt.WithSPChannel(fsl)),
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
	var res leaseproto.AskRep
	if err := lmc.cc.RPC(srv.String(), "LeaseSrv.AskLease",
		&leaseproto.AskReq{
			ClntId: uint64(lmc.ClntId()),
			TTL:    uint64(ttl),
		},
		&res); err == nil {
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
