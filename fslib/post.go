package fslib

import (
	"fmt"
	"strings"
	// "log"

	// db "ulambda/debug"
	np "ulambda/ninep"
)

func MakeTarget(srvaddrs []string) []byte {
	targets := []string{}
	for _, addr := range srvaddrs {
		targets = append(targets, addr+":pubkey")
	}
	return []byte(strings.Join(targets, "\n"))
}

func (fsl *FsLib) PostService(srvaddr, srvname string) error {
	err := fsl.Symlink(MakeTarget([]string{srvaddr}), srvname, 0777|np.DMTMP)
	return err
}

func (fsl *FsLib) PostServiceUnion(srvaddr, srvpath, server string) error {
	p := srvpath + "/" + server
	dir, err := fsl.IsDir(srvpath)
	if err != nil {
		return err
	}
	if !dir {
		return fmt.Errorf("Not a directory")
	}
	err = fsl.Symlink(MakeTarget([]string{srvaddr}), p, 0777|np.DMTMP)
	return err
}

func (fsl *FsLib) Post(srvaddr, path string) error {
	if np.EndSlash(path) {
		return fsl.PostServiceUnion(srvaddr, path, srvaddr)
	} else {
		return fsl.PostService(srvaddr, path)
	}
}
