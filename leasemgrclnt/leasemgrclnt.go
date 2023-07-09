package leasemgrclnt

import (
	"path"

	db "sigmaos/debug"
	"sigmaos/fsetcd"
	"sigmaos/fslib"
	leaseproto "sigmaos/lease/proto"
	"sigmaos/protdevclnt"
	sp "sigmaos/sigmap"
	"sigmaos/syncmap"
)

type LeaseMgrClnt struct {
	*fslib.FsLib
	lm *syncmap.SyncMap[string, sp.TleaseId]
}

func NewLeaseMgrClnt(fsl *fslib.FsLib) (*LeaseMgrClnt, error) {
	return &LeaseMgrClnt{FsLib: fsl, lm: syncmap.NewSyncMap[string, sp.TleaseId]()}, nil
}

// Ask for lease; if caller already has a lease at that server, return
// it.
func (lmc *LeaseMgrClnt) AskLease(pn string, ttl sp.Tttl) (sp.TleaseId, error) {
	db.DPrintf(db.LEASEMGRCLNT, "AskLease %v\n", pn)
	srv, rest, err := lmc.LastMount(pn, lmc.Uname())
	db.DPrintf(db.LEASEMGRCLNT, "AskLease %v: %v %v err %v\n", pn, srv, rest, err)
	if lid, ok := lmc.lm.Lookup(srv.String()); ok {
		return lid, nil
	}
	pdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{lmc.FsLib}, path.Join(srv.String(), sp.LEASESRV))
	if err != nil {
		return sp.NoLeaseId, err
	}
	var res leaseproto.AskResult
	if err := pdc.RPC("LeaseSrv.AskLease",
		&leaseproto.AskRequest{
			ClntId: uint64(lmc.ClntID()),
			TTL:    fsetcd.LeaseTTL}, &res); err != nil {
		return sp.TleaseId(res.LeaseId), err
	}
	return sp.NoLeaseId, nil
}

// Write KeepAlieve to lease ctl file
func (lmc *LeaseMgrClnt) keepAliveOnce(lid sp.TleaseId) {
}

func (lmc *LeaseMgrClnt) Revoke(lid sp.TleaseId) {
}

// Refreshes lid continously
func (lmc *LeaseMgrClnt) Refresher(lid sp.TleaseId) error {
	return nil
}
