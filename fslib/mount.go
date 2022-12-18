package fslib

import (
	"fmt"

	"sigmaos/fcall"
	"sigmaos/path"
	sp "sigmaos/sigmap"
	"sigmaos/union"
)

//
// Client side
//

// Return pn, replacing first ~ip/~any with a symlink for a specific
// server.
func (fsl *FsLib) ResolveUnion(pn string) (string, bool, error) {
	p := path.Split(pn)
	d, left, ok := p.IsUnion()
	if ok {
		n, _, err := fsl.resolveUnion(d, left[0])
		if err != nil {
			return "", false, err
		}
		left[0] = n
		return d + "/" + left.String() + "/", ok, nil
	}
	return "", ok, nil
}

// Return pn but with all ~ip and ~any's replaced with symlinks for a
// specific server.
func (fsl *FsLib) ResolveUnions(pn string) (string, error) {
	for {
		npn, ok, err := fsl.ResolveUnion(pn)
		if err != nil {
			return "", err
		}
		if !ok {
			return pn, nil
		}
		pn = npn
	}
}

// Make copy of root mount or first union mount in pn. Return the
// content of symlink and the symlink's name.
func (fsl *FsLib) CopyMount(pn string) (sp.Tmount, string, error) {
	if pn == sp.NAMED {
		return sp.MkMountService(Named()), "", nil
	}
	p := path.Split(pn)
	d, left, ok := p.IsUnion()
	if ok {
		_, mnt, err := fsl.resolveUnion(d, left[0])
		if err != nil {
			return sp.NullMount(), "", err
		}
		return mnt, left[1:].String(), nil
	}
	return sp.NullMount(), "", fcall.MkErr(fcall.TErrInval, pn)
}

// Return path to the symlink for the last server on this path and the
// the rest of the path on the server.
func (fsl *FsLib) PathLastSymlink(pn string) (string, path.Path, error) {
	// Make sure the server is automounted:
	if _, err := fsl.Stat(pn + "/"); err != nil {
		return "", nil, err
	}
	return fsl.LastMount(pn)
}

func (fsl *FsLib) resolveUnion(d string, q string) (string, sp.Tmount, error) {
	rmnt := sp.NullMount()
	rname := ""
	_, err := fsl.ProcessDir(d, func(st *sp.Stat) (bool, error) {
		b, err := fsl.GetFile(d + "/" + st.Name)
		if err != nil {
			return false, nil
		}
		mnt := sp.MkMount(b)
		if ok := union.UnionMatch(q, mnt.Mnt); ok {
			rname = st.Name
			rmnt = mnt
			return true, nil
		}
		return false, nil
	})
	if err == nil && rname != "" {
		return rname, rmnt, nil
	}
	return rname, rmnt, fcall.MkErr(fcall.TErrNotfound, d)
}

//
// Server side
//

func (fsl *FsLib) MountService(pn string, mnt sp.Tmount) error {
	return fsl.PutFileAtomic(pn, 0777|sp.DMTMP|sp.DMSYMLINK, []byte(mnt.Mnt))
}

// For code running using /mnt/9p, which doesn't support PutFile.
func (fsl *FsLib) MkMountSymlink9P(pn string, mnt sp.Tmount) error {
	return fsl.Symlink([]byte(mnt.Mnt), pn, 0777|sp.DMTMP)
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
		return fsl.MountServiceUnion(pn, mnt, sp.Address(mnt))
	} else {
		return fsl.MountService(pn, mnt)
	}
}
