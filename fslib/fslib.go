package fslib

import (
	db "sigmaos/debug"
	"sigmaos/proc"
	sos "sigmaos/sigmaos"
	sp "sigmaos/sigmap"
)

type FsLib struct {
	pcfg *proc.ProcEnv
	sos.SigmaOS
}

func NewFsLibAPI(pcfg *proc.ProcEnv, sos sos.SigmaOS) (*FsLib, error) {
	db.DPrintf(db.PORT, "NewFsLib: uname %s lip %s addrs %v\n", pcfg.GetUname(), pcfg.GetLocalIP(), pcfg.EtcdIP)
	fl := &FsLib{
		pcfg:    pcfg,
		SigmaOS: sos,
	}
	return fl, nil
}

func (fl *FsLib) GetLocalIP() sp.Thost {
	return fl.pcfg.GetLocalIP()
}

func (fl *FsLib) ProcEnv() *proc.ProcEnv {
	return fl.pcfg
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
