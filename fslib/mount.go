package fslib

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/union"
)

func (fsl *FsLib) MountService(pn string, mnt sp.Tmount, lid sp.TleaseId) error {
	b, err := mnt.Marshal()
	if err != nil {
		return err
	}
	if err := fsl.PutFileAtomic(pn, 0777|sp.DMSYMLINK, b, lid); err != nil {
		db.DPrintf(db.ALWAYS, "PutFileAtomic %v err %v\n", pn, err)
		return err
	}
	return nil
}

func (fsl *FsLib) MountServiceUnion(pn string, mnt sp.Tmount, name string, lid sp.TleaseId) error {
	p := pn + "/" + name
	dir, err := fsl.IsDir(pn)
	if err != nil {
		return err
	}
	if !dir {
		return fmt.Errorf("Not a directory")
	}
	return fsl.MountService(p, mnt, lid)
}

func (fsl *FsLib) NewMountSymlink(pn string, mnt sp.Tmount, lid sp.TleaseId) error {
	if path.EndSlash(pn) {
		return fsl.MountServiceUnion(pn, mnt, mnt.Address().Addr, lid)
	} else {
		return fsl.MountService(pn, mnt, lid)
	}
}

// Return pn, replacing first ~local/~any with a symlink for a specific
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

// Return pn but with all ~local and ~any's replaced with symlinks for a
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

func (fsl *FsLib) ReadMount(pn string) (sp.Tmount, error) {
	target, err := fsl.GetFile(pn)
	if err != nil {
		return sp.Tmount{}, err
	}
	mnt, error := sp.NewMount(target)
	if error != nil {
		return sp.Tmount{}, err
	}
	return mnt, err
}

// Make copy of root mount or first union mount in pn. Return the
// content of symlink and the symlink's name.
func (fsl *FsLib) CopyMount(pn string) (sp.Tmount, string, error) {
	if pn == sp.NAMED {
		return sp.NewMountService(fsl.GetMntNamed().Addr), "", nil
	}
	p := path.Split(pn)
	d, left, ok := p.IsUnion()
	if ok {
		_, mnt, err := fsl.resolveUnion(d, left[0])
		if err != nil {
			return sp.NullMount(), "", err
		}
		return mnt, left[1:].String(), nil
	} else if s, p, err := fsl.PathLastSymlink(pn); err == nil {
		if mnt, err := fsl.ReadMount(s.String()); err == nil {
			return mnt, p.String(), nil
		}
	}
	return sp.NullMount(), "", serr.NewErr(serr.TErrInval, pn)
}

func (fsl *FsLib) resolveUnion(d string, q string) (string, sp.Tmount, error) {
	rmnt := sp.NullMount()
	rname := ""
	// Make sure to resolve d in case it is a symlink or mount point.
	_, err := fsl.ProcessDir(d+"/", func(st *sp.Stat) (bool, error) {
		b, err := fsl.GetFile(d + "/" + st.Name)
		if err != nil {
			return false, nil
		}
		mnt, error := sp.NewMount(b)
		if error != nil {
			return false, nil
		}
		if ok := union.UnionMatch(fsl.GetLocalIP(), q, mnt); ok {
			rname = st.Name
			rmnt = mnt
			return true, nil
		}
		return false, nil
	})
	if err == nil && rname != "" {
		return rname, rmnt, nil
	}
	return rname, rmnt, serr.NewErr(serr.TErrNotfound, d)
}

// For code running using /mnt/9p, which doesn't support PutFile.
func (fsl *FsLib) NewMountSymlink9P(pn string, mnt sp.Tmount) error {
	b, err := mnt.Marshal()
	if err != nil {
		return err
	}
	return fsl.Symlink(b, pn, 0777|sp.DMTMP)
}
