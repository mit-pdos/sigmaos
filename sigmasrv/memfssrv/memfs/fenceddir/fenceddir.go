// Package fenceddir wraps fs.Dir with fence checking
package fenceddir

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/sigmasrv/fencefs"
	"sigmaos/api/fs"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type FencedDir struct {
	fs.Dir
}

func (fdir *FencedDir) String() string {
	return fmt.Sprintf("fenceddir %p %v", fdir, fdir.Dir)
}

func NewFencedRoot(root fs.Dir) fs.Dir {
	return &FencedDir{root}
}

func GetDir(dir fs.Dir) fs.Dir {
	switch d := dir.(type) {
	case *FencedDir:
		return d.Dir
	default:
		return d
	}
}

func (fdir *FencedDir) LookupPath(ctx fs.CtxI, path path.Tpathname) ([]fs.FsObj, fs.FsObj, path.Tpathname, *serr.Err) {
	os, lo, p, err := fdir.Dir.LookupPath(ctx, path)
	for i, o := range os {
		switch d := o.(type) {
		case fs.Dir:
			os[i] = &FencedDir{d}
		}
	}

	switch d := lo.(type) {
	case fs.Dir:
		db.DPrintf(db.FENCEFS, "Walk wrap %v %v", path, d)
		lo = &FencedDir{d}
	}

	if lo != nil {
		if d := lo.Parent(); d != nil {
			switch d.(type) {
			case *FencedDir:
			default:
				db.DPrintf(db.FENCEFS, "Parent wrap %v", d)
				lo.SetParent(&FencedDir{d})
			}
		}
	}

	return os, lo, p, err
}

func (fdir *FencedDir) Create(ctx fs.CtxI, name string, perm sp.Tperm, m sp.Tmode, lid sp.TleaseId, f sp.Tfence, dev fs.FsObj) (fs.FsObj, *serr.Err) {
	fi, err := fencefs.CheckFence(ctx.FenceFs(), f)
	if err != nil {
		db.DPrintf(db.FENCEFS, "Create %v %v %v fence err %v", fdir, name, f, err)
		return nil, err
	}
	if fi != nil {
		defer fi.RUnlock()
	}
	o, err := fdir.Dir.Create(ctx, name, perm, m, lid, f, dev)
	switch d := o.(type) {
	case fs.Dir:
		return &FencedDir{d}, err
	default:
		return o, err
	}
}

func (fdir *FencedDir) Open(ctx fs.CtxI, m sp.Tmode) (fs.FsObj, *serr.Err) {
	o, err := fdir.Dir.Open(ctx, m)
	switch d := o.(type) {
	case fs.Dir:
		return &FencedDir{d}, err
	default:
		return o, err
	}
}

func (fdir *FencedDir) Remove(ctx fs.CtxI, n string, f sp.Tfence, del fs.Tdel) *serr.Err {
	fi, err := fencefs.CheckFence(ctx.FenceFs(), f)
	if err != nil {
		db.DPrintf(db.FENCEFS, "Remove %v %v %v fence err %v", fdir, n, f, err)
		return err
	}
	if fi != nil {
		defer fi.RUnlock()
	}
	return fdir.Dir.Remove(ctx, n, f, del)
}

func (fdir *FencedDir) Rename(ctx fs.CtxI, from, to string, f sp.Tfence) *serr.Err {
	fi, err := fencefs.CheckFence(ctx.FenceFs(), f)
	if err != nil {
		db.DPrintf(db.FENCEFS, "Rename %v %v %v fence err %v", fdir, from, to, err)
		return err
	}
	if fi != nil {
		defer fi.RUnlock()
	}
	return fdir.Dir.Rename(ctx, from, to, f)
}

func (fdir *FencedDir) Renameat(ctx fs.CtxI, old string, nd fs.Dir, new string, f sp.Tfence) *serr.Err {
	fi, err := fencefs.CheckFence(ctx.FenceFs(), f)
	if err != nil {
		db.DPrintf(db.FENCEFS, "Renameat %v %v %v fence err %v", fdir, old, new, err)
		return err
	}
	if fi != nil {
		defer fi.RUnlock()
	}
	switch d := nd.(type) {
	case *FencedDir:
		return fdir.Dir.Renameat(ctx, old, d.Dir, new, f)
	default:
		db.DFatalf("Rename: type err %T", d)
		return nil
	}
}
