package fslib

import (
	"log"
	"os"
	"strings"

	"ulambda/fsclnt"
	np "ulambda/ninep"
	"ulambda/proc"
)

type FsLib struct {
	*fsclnt.FsClient
	chunkSz np.Tsize
}

func (fl *FsLib) SetChunkSz(sz np.Tsize) {
	fl.chunkSz = sz
}

func NamedAddr() string {
	named := os.Getenv("NAMED")
	if named == "" {
		log.Fatal("FATAL Getenv error: missing NAMED")
	}
	return named
}

func Named() []string {
	nameds := strings.Split(NamedAddr(), ",")
	return nameds
}

func MakeFsLibBase(uname string) *FsLib {
	// Picking a small chunk size really kills throughput
	return &FsLib{fsclnt.MakeFsClient(uname), np.Tsize(10_000_000)}
}

func (fl *FsLib) MountTree(server []string, tree, mount string) error {
	if fd, err := fl.AttachReplicas(server, "", tree); err == nil {
		return fl.Mount(fd, mount)
	} else {
		return err
	}
}

func MakeFsLibAddr(uname string, server []string) *FsLib {
	fl := MakeFsLibBase(uname)
	err := fl.MountTree(server, "", "name")
	if err != nil {
		log.Fatalf("FATAL %v: Mount %v error: %v", proc.GetProgram(), server, err)
	}
	return fl
}

func MakeFsLib(uname string) *FsLib {
	return MakeFsLibAddr(uname, Named())
}
