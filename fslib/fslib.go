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

func MakeFsLibAddrNet(uname string, realm sp.Trealm, lip string, addrs sp.Taddrs, clntnet string) (*FsLib, error) {
	db.DPrintf(db.PORT, "MakeFsLibAddrRealm: uname %s lip %s addrs %v\n", uname, lip, addrs)
	fl := &FsLib{fdclnt.MakeFdClient(nil, uname, clntnet, realm, lip, sessp.Tsize(10_000_000)), addrs}
	return fl, nil
}

func MakeFsLibAddr(uname string, realm sp.Trealm, lip string, addrs sp.Taddrs) (*FsLib, error) {
	return MakeFsLibAddrNet(uname, realm, lip, addrs, proc.GetNet())
}

// Only to be called by procs.
func MakeFsLib(uname string) (*FsLib, error) {
	as, err := proc.Named()
	if err != nil {
		return nil, err
	}
	return MakeFsLibAddr(uname, proc.GetRealm(), proc.GetSigmaLocal(), as)
}

func (fl *FsLib) NamedAddr() sp.Taddrs {
	mnt := fl.GetMntNamed()
	return mnt.Addr
}

func (fl *FsLib) MountTree(addrs sp.Taddrs, tree, mount string) error {
	if fd, err := fl.Attach(fl.Uname(), addrs, "", tree); err == nil {
		return fl.Mount(fd, mount)
	} else {
		return err
	}
}

func (fl *FsLib) Exit() error {
	return fl.PathClnt.Exit()
}
