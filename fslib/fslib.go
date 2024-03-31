package fslib

import (
	db "sigmaos/debug"
	"sigmaos/netsigma"
	"sigmaos/proc"
	sos "sigmaos/sigmaos"
	sp "sigmaos/sigmap"
)

type FsLib struct {
	pe  *proc.ProcEnv
	npc *netsigma.NetProxyClnt
	sos.SigmaOS
}

func NewFsLibAPI(pe *proc.ProcEnv, npc *netsigma.NetProxyClnt, sos sos.SigmaOS) (*FsLib, error) {
	db.DPrintf(db.FSLIB, "NewFsLib: principal %s innerip %s addrs %v\n", pe.GetPrincipal(), pe.GetInnerContainerIP(), pe.EtcdIP)
	fl := &FsLib{
		pe:      pe,
		npc:     npc,
		SigmaOS: sos,
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
func (fl *FsLib) GetNetProxyClnt() *netsigma.NetProxyClnt {
	return fl.npc
}

func (fl *FsLib) MountTree(addrs sp.Taddrs, tree, mount string) error {
	return fl.SigmaOS.MountTree(addrs, tree, mount)
}

func (fl *FsLib) Close() error {
	return fl.SigmaOS.Close()
}

func (fl *FsLib) GetSigmaOS() sos.SigmaOS {
	return fl.SigmaOS
}
