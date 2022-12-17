package fslib

import (
	"fmt"
	"strings"
	// "log"

	// db "sigmaos/debug"
	"sigmaos/path"
	np "sigmaos/sigmap"
)

//
// XXX old interface
//

func MakeTarget(srvaddrs []string) []byte {
	targets := []string{}
	for _, addr := range srvaddrs {
		targets = append(targets, addr+":pubkey")
	}
	return []byte(strings.Join(targets, "\n"))
}

func MakeTargetTree(srvaddr string, tree path.Path) []byte {
	target := []string{srvaddr, "pubkey", tree.String()}
	return []byte(strings.Join(target, ":"))
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

func (fsl *FsLib) Post(srvaddr, pn string) error {
	if path.EndSlash(pn) {
		return fsl.PostServiceUnion(srvaddr, pn, srvaddr)
	} else {
		return fsl.PostService(srvaddr, pn)
	}
}

//
// tweaked, new  interface
//

// XXX introduce mount type
func address(mnt []byte) string {
	targets := strings.Split(string(mnt), "\n")
	if strings.HasPrefix(targets[0], "[") {
		parts := strings.SplitN(targets[0], ":", 6)
		return "[" + parts[0] + ":" + parts[1] + ":" + parts[2] + "]" + ":" + parts[3]
	} else { // IPv4
		parts := strings.SplitN(targets[0], ":", 4)
		return parts[0] + ":" + parts[1]
	}
}

func MkMountService(srvaddrs []string) []byte {
	targets := []string{}
	for _, addr := range srvaddrs {
		targets = append(targets, addr+":pubkey")
	}
	return []byte(strings.Join(targets, "\n"))
}

func MkMountServer(addr string) []byte {
	return MkMountService([]string{addr})
}

func MkMountTree(mount []byte, tree string) []byte {
	target := []string{string(mount), tree}
	return []byte(strings.Join(target, ":"))
}

func (fsl *FsLib) MountService(pn string, mount []byte) error {
	err := fsl.Symlink(mount, pn, 0777|np.DMTMP)
	return err
}

func (fsl *FsLib) MountServiceUnion(pn string, mnt []byte, name string) error {
	p := pn + "/" + name
	dir, err := fsl.IsDir(pn)
	if err != nil {
		return err
	}
	if !dir {
		return fmt.Errorf("Not a directory")
	}
	err = fsl.Symlink(mnt, p, 0777|np.DMTMP)
	return err
}

func (fsl *FsLib) MkMount(pn string, mount []byte) error {
	if path.EndSlash(pn) {
		return fsl.MountServiceUnion(pn, mount, address(mount))
	} else {
		return fsl.MountService(pn, mount)
	}
}
