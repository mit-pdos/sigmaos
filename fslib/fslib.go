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
	realm     sp.Trealm
	namedAddr sp.Taddrs
}

func MakeFsLibAddrNet(uname string, realm sp.Trealm, lip string, addrs sp.Taddrs, clntnet string) (*FsLib, error) {
	db.DPrintf(db.PORT, "MakeFsLibAddrRealm: uname %s lip %s addrs %v\n", uname, lip, addrs)
	fl := &FsLib{fdclnt.MakeFdClient(nil, uname, clntnet, lip, sessp.Tsize(10_000_000)), realm, addrs}
	if err := fl.MountTree(addrs, "", "name"); err != nil {
		return nil, err
	}
	return fl, nil
}

func MakeFsLibAddr(uname string, realm sp.Trealm, lip string, addrs sp.Taddrs) (*FsLib, error) {
	return MakeFsLibAddrNet(uname, realm, lip, addrs, proc.GetNet())
}

// Only to be called by procs.
func MakeFsLib(uname string) (*FsLib, error) {
	return MakeFsLibAddr(uname, proc.GetRealm(), proc.GetSigmaLocal(), proc.Named())
}

func (fl *FsLib) NamedAddr() sp.Taddrs {
	return fl.namedAddr
}

func (fl *FsLib) Realm() sp.Trealm {
	return fl.realm
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
