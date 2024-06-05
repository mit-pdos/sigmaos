package fslib

import (
	db "sigmaos/debug"
	"sigmaos/netproxyclnt"
	"sigmaos/proc"
	sos "sigmaos/sigmaos"
	sp "sigmaos/sigmap"
)

type FsLib struct {
	pe  *proc.ProcEnv
	npc *netproxyclnt.NetProxyClnt
	sos.FileAPI
}

func NewFsLibAPI(pe *proc.ProcEnv, npc *netproxyclnt.NetProxyClnt, sos sos.FileAPI) (*FsLib, error) {
	db.DPrintf(db.FSLIB, "NewFsLib: principal %s innerip %s addrs %v\n", pe.GetPrincipal(), pe.GetInnerContainerIP(), pe.GetEtcdEndpoints())
	fl := &FsLib{
		pe:      pe,
		npc:     npc,
		FileAPI: sos,
	}
	return fl, nil
}

func (fl *FsLib) GetInnerContainerIP() sp.Tip {
	return fl.pe.GetInnerContainerIP()
}

func (fl *FsLib) ProcEnv() *proc.ProcEnv {
	return fl.pe
}

// TODO: should probably remove, and replace by a high-level SigmaOS API call.
func (fl *FsLib) GetNetProxyClnt() *netproxyclnt.NetProxyClnt {
	return fl.npc
}

func (fl *FsLib) MountTree(ep *sp.Tendpoint, tree, mount string) error {
	return fl.FileAPI.MountTree(ep, tree, mount)
}

func (fl *FsLib) Close() error {
	return fl.FileAPI.Close()
}

func (fl *FsLib) GetSigmaOS() sos.FileAPI {
	return fl.FileAPI
}
