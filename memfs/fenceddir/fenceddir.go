package fenceddir

import (
	"fmt"

	db "sigmaos/debug"
	"sigmaos/fencefs"
	"sigmaos/fs"
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

func (fdir *FencedDir) LookupPath(ctx fs.CtxI, path path.Tpathname) ([]fs.FsObj, fs.FsObj, path.Tpathname, *serr.Err) {
	db.DPrintf(db.FENCEFS, "LookupPath %v %v", fdir, path)
	os, lo, p, err := fdir.Dir.LookupPath(ctx, path)
	for i, o := range os {
		switch d := o.(type) {
		case fs.Dir:
			db.DPrintf(db.FENCEFS, "Walk wrap %v %v", path, d)
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
				db.DPrintf(db.FENCEFS, "Parent is already wrapped %v", d)
			default:
				lo.SetParent(&FencedDir{d})
			}
		}
	}

	return os, lo, p, err
}

func (fdir *FencedDir) Create(ctx fs.CtxI, name string, perm sp.Tperm, m sp.Tmode, lid sp.TleaseId, f sp.Tfence, dev fs.FsObj) (fs.FsObj, *serr.Err) {
	db.DPrintf(db.FENCEFS, "Create %v %v", fdir, name)
	o, err := fdir.Dir.Create(ctx, name, perm, m, lid, f, dev)
	switch d := o.(type) {
	case fs.Dir:
		db.DPrintf(db.FENCEFS, "Create wrap %v %v", name, d)
		return &FencedDir{d}, err
	default:
		return o, err
	}
}

func (fdir *FencedDir) Open(ctx fs.CtxI, m sp.Tmode) (fs.FsObj, *serr.Err) {
	db.DPrintf(db.FENCEFS, "Open %v", fdir)
	o, err := fdir.Dir.Open(ctx, m)
	switch d := o.(type) {
	case fs.Dir:
		return &FencedDir{d}, err
	default:
		return o, err
	}
}

func (fdir *FencedDir) Remove(ctx fs.CtxI, n string, f sp.Tfence, del fs.Tdel) *serr.Err {
	db.DPrintf(db.FENCEFS, "Remove %v %v", fdir, n)
	return fdir.Dir.Remove(ctx, n, f, del)
}

func (fdir *FencedDir) Rename(ctx fs.CtxI, from, to string, f sp.Tfence) *serr.Err {
	db.DPrintf(db.FENCEFS, "Rename %v %v %v", fdir, from, to)
	if _, err := fencefs.CheckFence(ctx.FenceFs(), f); err != nil {
		return err
	}
	return fdir.Dir.Rename(ctx, from, to, f)
}
