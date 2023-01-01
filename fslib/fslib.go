package fslib

import (
	"net"
	"os"
	"runtime/debug"
	"strings"

	db "sigmaos/debug"
	"sigmaos/fdclnt"
	"sigmaos/sessp"
)

type FsLib struct {
	*fdclnt.FdClient
}

func NamedAddrs() string {
	addrs := os.Getenv("SIGMANAMED")
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

func SetNamedIP(ip string) ([]string, error) {
	nameds := Named()
	for i, s := range nameds {
		_, port, err := net.SplitHostPort(s)
		if err != nil {
			return nil, err
		}
		nameds[i] = net.JoinHostPort(ip, port)
	}
	return nameds, nil
}

func MakeFsLibBase(uname string) *FsLib {
	// Picking a small chunk size really kills throughput
	return &FsLib{fdclnt.MakeFdClient(nil, uname, sessp.Tsize(10_000_000))}
}

func (fl *FsLib) MountTree(addrs []string, tree, mount string) error {
	if fd, err := fl.Attach(fl.Uname(), addrs, "", tree); err == nil {
		return fl.Mount(fd, mount)
	} else {
		return err
	}
}

func MakeFsLibAddr(uname string, addrs []string) (*FsLib, error) {
	fl := MakeFsLibBase(uname)
	err := fl.MountTree(addrs, "", "name")
	if err != nil {
		debug.PrintStack()
		return nil, err
	}
	return fl, nil
}

func MakeFsLib(uname string) (*FsLib, error) {
	return MakeFsLibAddr(uname, Named())
}

func (fl *FsLib) Exit() error {
	return fl.PathClnt.Exit()
}
