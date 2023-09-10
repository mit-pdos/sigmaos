package fslib

import (
	"sigmaos/proc"
	db "sigmaos/debug"
	"sigmaos/fdclnt"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type FsLib struct {
	scfg *proc.ProcEnv
	*fdclnt.FdClient
}

// Only to be called by procs.
func MakeFsLib(scfg *proc.ProcEnv) (*FsLib, error) {
	db.DPrintf(db.PORT, "MakeFsLib: uname %s lip %s addrs %v\n", scfg.Uname, scfg.LocalIP, scfg.EtcdIP)
	fl := &FsLib{
		scfg:     scfg,
		FdClient: fdclnt.MakeFdClient(scfg, nil, sessp.Tsize(10_000_000)),
	}
	return fl, nil
}

func (fl *FsLib) ProcEnv() *proc.ProcEnv {
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
