package fslib

import (
	"sigmaos/config"
	db "sigmaos/debug"
	"sigmaos/fdclnt"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type FsLib struct {
	scfg *config.SigmaConfig
	*fdclnt.FdClient
}

// Only to be called by procs.
func MakeFsLib(scfg *config.SigmaConfig) (*FsLib, error) {
	db.DPrintf(db.PORT, "MakeFsLib: uname %s lip %s addrs %v\n", scfg.Uname, scfg.LocalIP, scfg.EtcdAddr)
	fl := &FsLib{
		scfg:     scfg,
		FdClient: fdclnt.MakeFdClient(scfg, nil, sessp.Tsize(10_000_000)),
	}
	return fl, nil
}

func (fl *FsLib) SigmaConfig() *config.SigmaConfig {
	return fl.scfg
}

func (fl *FsLib) NamedAddr() sp.Taddrs {
	mnt := fl.GetMntNamed(fl.Uname())
	return mnt.Addr
}

func (fl *FsLib) MountTree(addrs sp.Taddrs, tree, mount string) error {
	return fl.FdClient.MountTree(fl.Uname(), addrs, tree, mount)
}

func (fl *FsLib) DetachAll() error {
	return fl.PathClnt.DetachAll()
}
