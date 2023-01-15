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
)

type FsLib struct {
	*fdclnt.FdClient
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
	addrs := strings.Split(NamedAddrs(), ",")
	return addrs
}

// XXX move to proc/env.go?
func SetSigmaNamed(nds []string) {
	s := strings.Join(nds, ",")
	os.Setenv(proc.SIGMANAMED, s)
}

func SetNamedIP(ip string, ports []string) ([]string, error) {
	nameds := make([]string, len(ports))
	for i, s := range ports {
		_, port, err := net.SplitHostPort(s)
		if err != nil {
			return nil, err
		}
		nameds[i] = net.JoinHostPort(ip, port)
	}
	return nameds, nil
}

func MakeFsLibBase(uname, lip string) *FsLib {
	// Picking a small chunk size really kills throughput
	return &FsLib{fdclnt.MakeFdClient(nil, uname, lip, sessp.Tsize(10_000_000))}
}

func (fl *FsLib) MountTree(addrs []string, tree, mount string) error {
	if fd, err := fl.Attach(fl.Uname(), addrs, "", tree); err == nil {
		return fl.Mount(fd, mount)
	} else {
		return err
	}
}

func MakeFsLibAddr(uname string, lip string, addrs []string) (*FsLib, error) {
	fl := MakeFsLibBase(uname, lip)
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

func (fl *FsLib) Exit() error {
	return fl.PathClnt.Exit()
}
