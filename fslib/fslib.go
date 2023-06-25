package fslib

import (
	db "sigmaos/debug"
	"sigmaos/fdclnt"
	"sigmaos/proc"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type FsLib struct {
	*fdclnt.FdClient
	namedAddr sp.Taddrs
}

func MakeFsLibAddrNet(uname sp.Tuname, realm sp.Trealm, lip string, addrs sp.Taddrs, clntnet string) (*FsLib, error) {
	db.DPrintf(db.PORT, "MakeFsLibAddrRealm: uname %s lip %s addrs %v\n", uname, lip, addrs)
	fl := &FsLib{fdclnt.MakeFdClient(nil, uname, clntnet, realm, lip, sessp.Tsize(10_000_000)), addrs}
	return fl, nil
}

func MakeFsLibAddr(uname sp.Tuname, realm sp.Trealm, lip string, addrs sp.Taddrs) (*FsLib, error) {
	return MakeFsLibAddrNet(uname, realm, lip, addrs, proc.GetNet())
}

// Only to be called by procs.
func MakeFsLib(uname sp.Tuname) (*FsLib, error) {
	as, err := proc.Named()
	if err != nil {
		return nil, err
	}
	return MakeFsLibAddr(uname, proc.GetRealm(), proc.GetSigmaLocal(), as)
}

func (fl *FsLib) NamedAddr() sp.Taddrs {
	mnt := fl.GetMntNamed(fl.Uname())
	return mnt.Addr
}

func (fl *FsLib) MountTree(addrs sp.Taddrs, tree, mount string) error {
	return fl.FdClient.MountTree(fl.Uname(), addrs, tree, mount)
}

func (fl *FsLib) Exit() error {
	db.DPrintf(db.ALWAYS, "%v: exit fslib\n", proc.GetPid())
	return fl.PathClnt.Exit()
}
