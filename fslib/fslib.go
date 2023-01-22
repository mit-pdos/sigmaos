package fslib

import (
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

func MakeFsLibBase(uname string, realm sp.Trealm, lip string, namedAddr []string) *FsLib {
	// Picking a small chunk size really kills throughput
	return &FsLib{fdclnt.MakeFdClient(nil, uname, lip, sessp.Tsize(10_000_000)),
		realm, namedAddr}
}

func (fl *FsLib) MountTree(addrs []string, tree, mount string) error {
	if fd, err := fl.Attach(fl.Uname(), addrs, "", tree); err == nil {
		return fl.Mount(fd, mount)
	} else {
		return err
	}
}

func MakeFsLibRealmAddr(uname string, r sp.Trealm, lip string, addrs []string) (*FsLib, error) {
	fl := MakeFsLibBase(uname, r, lip, addrs)
	err := fl.MountTree(addrs, "", "name")
	if err != nil {
		return nil, err
	}
	return fl, nil
}

// get realm from "caller"
func MakeFsLibAddr(uname, lip string, addrs []string) (*FsLib, error) {
	fl := MakeFsLibBase(uname, sp.ROOTREALM, lip, addrs)
	err := fl.MountTree(addrs, "", "name")
	if err != nil {
		return nil, err
	}
	return fl, nil
}

func MakeFsLibNamed(uname string, addrs []string) (*FsLib, error) {
	return MakeFsLibAddr(uname, proc.GetSigmaLocal(), addrs)
}

func MakeFsLib(uname string) (*FsLib, error) {
	return MakeFsLibNamed(uname, proc.Named())
}

func (fl *FsLib) NamedAddr() sp.Taddrs {
	return fl.namedAddr
}

func (fl *FsLib) Realm() sp.Trealm {
	return fl.realm
}

func (fl *FsLib) Exit() error {
	return fl.PathClnt.Exit()
}
