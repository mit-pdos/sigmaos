package ndclnt

import (
	db "sigmaos/debug"
	"sigmaos/ft/procgroupmgr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

type NdMgr struct {
	*NdClnt
	cfg *procgroupmgr.ProcGroupMgrConfig
	grp *procgroupmgr.ProcGroupMgr
}

func NewNdGrpMgr(sc *sigmaclnt.SigmaClnt, realm sp.Trealm, cfg *procgroupmgr.ProcGroupMgrConfig, clear bool) (*NdMgr, error) {
	ndc, err := NewNdClnt(sc, realm)
	if err != nil {
		return nil, err
	}
	if clear {
		err := ndc.RemoveNamedEP()
		db.DPrintf(db.NAMED_LDR, "RealmSrv.Make %v rm named ep err %v", ndc.pn, err)
	}
	return &NdMgr{
		NdClnt: ndc,
		cfg:    cfg,
	}, nil
}

func (ndg *NdMgr) Grp() *procgroupmgr.ProcGroupMgr {
	return ndg.grp
}

func (ndg *NdMgr) Cfg() *procgroupmgr.ProcGroupMgrConfig {
	return ndg.cfg
}

func (ndg *NdMgr) StartNamedGrp() error {
	db.DPrintf(db.NAMED_LDR, "StartNamedGrp %v spawn named", ndg.cfg)
	ndg.grp = ndg.cfg.StartGrpMgr(ndg.scRoot)
	return nil
}

func (ndg *NdMgr) StopNamedGrp() error {
	if _, err := ndg.grp.StopGroup(); err != nil {
		return err
	}
	return nil
}
