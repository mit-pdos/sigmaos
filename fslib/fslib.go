package fslib

import (
	"os"
	"strings"

	db "ulambda/debug"
	"ulambda/fdclnt"
	np "ulambda/ninep"
	"ulambda/proc"
)

type FsLib struct {
	*fdclnt.FdClient
}

func NamedAddr() string {
	named := os.Getenv("NAMED")
	if named == "" {
		db.DFatalf("FATAL Getenv error: missing NAMED")
	}
	return named
}

func Named() []string {
	nameds := strings.Split(NamedAddr(), ",")
	return nameds
}

func MakeFsLibBase(uname string) *FsLib {
	// Picking a small chunk size really kills throughput
	return &FsLib{fdclnt.MakeFdClient(nil, uname, np.Tsize(10_000_000))}
}

func (fl *FsLib) MountTree(server []string, tree, mount string) error {
	if fd, err := fl.Attach(fl.Uname(), server, "", tree); err == nil {
		return fl.Mount(fd, mount)
	} else {
		return err
	}
}

func MakeFsLibAddr(uname string, server []string) *FsLib {
	fl := MakeFsLibBase(uname)
	err := fl.MountTree(server, "", "name")
	if err != nil {
		db.DFatalf("FATAL %v: Mount %v error: %v", proc.GetProgram(), server, err)
	}
	return fl
}

func MakeFsLib(uname string) *FsLib {
	return MakeFsLibAddr(uname, Named())
}

func (fl *FsLib) Shutdown() error {
	return fl.PathClnt.Shutdown()
}
