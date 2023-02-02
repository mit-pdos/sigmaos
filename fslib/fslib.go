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

func MakeFsLibBase(uname string, realm sp.Trealm, lip string, namedAddr sp.Taddrs) *FsLib {
	// Picking a small chunk size really kills throughput
	return &FsLib{fdclnt.MakeFdClient(nil, uname, lip, sessp.Tsize(10_000_000)),
		realm, namedAddr}
}

func MakeFsLibAddr(uname string, r sp.Trealm, lip string, addrs sp.Taddrs) (*FsLib, error) {
	fl := MakeFsLibBase(uname, r, lip, addrs)
	err := fl.MountTree(addrs, "", "name")
	if err != nil {
		return nil, err
	}
	return fl, nil
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

func (fl *FsLib) MountTree(addrs []string, tree, mount string) error {
	if fd, err := fl.Attach(fl.Uname(), addrs, "", tree); err == nil {
		return fl.Mount(fd, mount)
	} else {
		return err
	}
}

func (fl *FsLib) Exit() error {
	return fl.PathClnt.Exit()
}
