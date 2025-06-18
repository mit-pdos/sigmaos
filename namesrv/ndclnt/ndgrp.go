package ndclnt

import (
	db "sigmaos/debug"
	"sigmaos/ft/procgroupmgr"
	"sigmaos/sigmaclnt"
)

type NdMgr struct {
	*NdClnt
	cfg *procgroupmgr.ProcGroupMgrConfig
	grp *procgroupmgr.ProcGroupMgr
}

func NewNdGrpMgr(sc *sigmaclnt.SigmaClnt, pn string, cfg *procgroupmgr.ProcGroupMgrConfig, clear bool) *NdMgr {
	ndc := NewNdClnt(sc, pn)
	if clear {
		err := ndc.RemoveNamedEP()
		db.DPrintf(db.NAMED_LDR, "RealmSrv.Make %v rm named ep err %v", ndc.pn, err)
	}
	return &NdMgr{
		NdClnt: ndc,
		cfg:    cfg,
	}
}

func (ndg *NdMgr) StartNamedGrp() error {
	db.DPrintf(db.NAMED_LDR, "StartNamedGrp %v spawn named", ndg.cfg)
	ndg.grp = ndg.cfg.StartGrpMgr(ndg.sc)
	return nil
}

func (ndg *NdMgr) StopNamedGrp() error {
	if _, err := ndg.grp.StopGroup(); err != nil {
		return err
	}
	return nil
}
