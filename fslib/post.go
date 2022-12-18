package fslib

import (
	"fmt"
	"strings"
	// "log"

	// db "sigmaos/debug"
	"sigmaos/path"
	sp "sigmaos/sigmap"
)

// XXX introduce mount type
func address(mnt sp.Tmount) string {
	targets := strings.Split(string(mnt.Mnt), "\n")
	if strings.HasPrefix(targets[0], "[") {
		parts := strings.SplitN(targets[0], ":", 6)
		return "[" + parts[0] + ":" + parts[1] + ":" + parts[2] + "]" + ":" + parts[3]
	} else { // IPv4
		parts := strings.SplitN(targets[0], ":", 4)
		return parts[0] + ":" + parts[1]
	}
}

func MkMountService(srvaddrs []string) sp.Tmount {
	targets := []string{}
	for _, addr := range srvaddrs {
		targets = append(targets, addr+":pubkey")
	}
	return sp.Tmount{strings.Join(targets, "\n")}
}

func MkMountServer(addr string) sp.Tmount {
	return MkMountService([]string{addr})
}

func MkMountTree(mnt sp.Tmount, tree string) sp.Tmount {
	target := []string{string(mnt.Mnt), tree}
	return sp.Tmount{strings.Join(target, ":")}
}

func (fsl *FsLib) MountService(pn string, mnt sp.Tmount) error {
	return fsl.PutFileAtomic(pn, 0777|sp.DMTMP|sp.DMSYMLINK, []byte(mnt.Mnt))
}

func (fsl *FsLib) MountServiceUnion(pn string, mnt sp.Tmount, name string) error {
	p := pn + "/" + name
	dir, err := fsl.IsDir(pn)
	if err != nil {
		return err
	}
	if !dir {
		return fmt.Errorf("Not a directory")
	}
	err = fsl.Symlink([]byte(mnt.Mnt), p, 0777|sp.DMTMP)
	return err
}

func (fsl *FsLib) MkMountSymlink(pn string, mnt sp.Tmount) error {
	if path.EndSlash(pn) {
		return fsl.MountServiceUnion(pn, mnt, address(mnt))
	} else {
		return fsl.MountService(pn, mnt)
	}
}
