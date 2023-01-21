package fslib

import (
	"net"
	"os"
	"runtime/debug"
	"strings"

	db "sigmaos/debug"
	"sigmaos/fdclnt"
	"sigmaos/proc"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type FsLib struct {
	*fdclnt.FdClient
	realm     sp.Trealm
	namedAddr []string
}

func NamedAddrs() string {
	addrs := proc.GetSigmaNamed()
	if addrs == "" {
		debug.PrintStack()
		db.DFatalf("Getenv error: missing SIGMANAMED")
	}
	return addrs
}

func Named() []string {
	return StringToNamedAddrs(NamedAddrs())
}

// XXX move to proc/env.go?
func SetSigmaNamed(nds []string) {
	s := strings.Join(nds, ",")
	os.Setenv(proc.SIGMANAMED, s)
}

func SetNamedIP(ip string, ports []string) ([]string, error) {
	nameds := make([]string, len(ports))
	for i, s := range ports {
		host, port, err := net.SplitHostPort(s)
		if err != nil {
			return nil, err
		}
		if host != "" {
			db.DFatalf("Tried to substitute named ip when port exists: %v -> %v %v", s, host, port)
		}
		nameds[i] = net.JoinHostPort(ip, port)
	}
	return nameds, nil
}

// XXX clean up.
func NamedAddrsToString(addrs []string) string {
	return strings.Join(addrs, ",")
}

func StringToNamedAddrs(s string) []string {
	return strings.Split(s, ",")
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
	return MakeFsLibNamed(uname, Named())
}

func (fl *FsLib) NamedAddr() []string {
	return fl.namedAddr
}

func (fl *FsLib) Exit() error {
	return fl.PathClnt.Exit()
}
