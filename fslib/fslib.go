package fslib

import (
	"os"
	"runtime/debug"
	"strings"

	db "sigmaos/debug"
	"sigmaos/fdclnt"
	np "sigmaos/ninep"
	"sigmaos/proc"
)

type FsLib struct {
	*fdclnt.FdClient
}

func NamedAddrs() string {
	addrs := os.Getenv("NAMED")
	if addrs == "" {
		db.DFatalf("Getenv error: missing NAMED")
	}
	return addrs
}

func Named() []string {
	addrs := strings.Split(NamedAddrs(), ",")
	return addrs
}

func MakeFsLibBase(uname string) *FsLib {
	// Picking a small chunk size really kills throughput
	return &FsLib{fdclnt.MakeFdClient(nil, uname, np.Tsize(10_000_000))}
}

func (fl *FsLib) MountTree(addrs []string, tree, mount string) error {
	if fd, err := fl.Attach(fl.Uname(), addrs, "", tree); err == nil {
		return fl.Mount(fd, mount)
	} else {
		return err
	}
}

func MakeFsLibAddr(uname string, addrs []string) *FsLib {
	fl := MakeFsLibBase(uname)
	err := fl.MountTree(addrs, "", "name")
	if err != nil {
		debug.PrintStack()
		db.DFatalf("%v: Mount %v error: %v", proc.GetProgram(), addrs, err)
	}
	return fl
}

func MakeFsLib(uname string) *FsLib {
	return MakeFsLibAddr(uname, Named())
}

func (fl *FsLib) Exit() error {
	return fl.PathClnt.Exit()
}
