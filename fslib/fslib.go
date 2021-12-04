package fslib

import (
	"log"
	"os"
	"strings"

	db "ulambda/debug"
	"ulambda/fsclnt"
)

type FsLib struct {
	*fsclnt.FsClient
}

func NamedAddr() string {
	named := os.Getenv("NAMED")
	if named == "" {
		log.Fatal("Getenv error: missing NAMED")
	}
	return named
}

func Named() []string {
	nameds := strings.Split(NamedAddr(), ",")
	return nameds
}

func MakeFsLibBase(uname string) *FsLib {
	db.Name(uname)
	return &FsLib{fsclnt.MakeFsClient(uname)}
}

func (fl *FsLib) MountTree(server []string, tree, mount string) error {
	if fd, err := fl.AttachReplicas(server, "", tree); err == nil {
		return fl.Mount(fd, mount)
	} else {
		return err
	}
}

// XXX not mounting "name" in named is a hack
func MakeFsLibAddr(uname string, server []string) *FsLib {
	fl := MakeFsLibBase(uname)
	if uname != "named" {
		err := fl.MountTree(server, "", "name")
		if err != nil {
			log.Fatalf("%v: Mount %v error: %v", db.GetName(), server, err)
		}
	}
	return fl
}

func MakeFsLib(uname string) *FsLib {
	return MakeFsLibAddr(uname, Named())
}
