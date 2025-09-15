package namesrv

import (
	"time"

	"sigmaos/api/fs"
	db "sigmaos/debug"
	"sigmaos/namesrv/fsetcd"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type Dir struct {
	*Obj
}

func (d *Dir) String() string {
	return d.Obj.String()
}

func newDir(o *Obj) *Dir {
	dir := &Dir{Obj: o}
	return dir
}

func (d *Dir) LookupPath(ctx fs.CtxI, pn path.Tpathname) ([]fs.FsObj, fs.FsObj, path.Tpathname, *serr.Err) {
	s := time.Now()
	if db.WillBePrinted(db.NAMED) {
		db.DPrintf(db.NAMED, "%v: Dir.Lookup %v o %v", ctx.ClntId(), pn, d)
	}
	name := pn[0]
	if path.IsUnionElem(pn[0]) {
		return nil, nil, pn, serr.NewErr(serr.TErrNotfound, name)
	}
	pn1 := d.pn.Copy().Append(name)
	di, err := d.fs.Lookup(&d.Obj.di, pn1)
	if err == nil {
		obj := newObjDi(d.fs, pn1, *di)
		var o fs.FsObj
		if obj.di.Perm.IsDir() {
			o = newDir(obj)
		} else if obj.di.Perm.IsDevice() {
			o = newDev(obj)
		} else {
			o = newFile(obj)
		}
		if db.WillBePrinted(db.WALK_LAT) {
			db.DPrintf(db.WALK_LAT, "Lookup %v %q %v lat %v", ctx.ClntId(), name, d, time.Since(s))
		}

		return []fs.FsObj{o}, o, pn[1:], nil
	}
	return nil, nil, pn, err
}

func (d *Dir) Create(ctx fs.CtxI, name string, perm sp.Tperm, m sp.Tmode, lid sp.TleaseId, f sp.Tfence, dev fs.FsObj) (fs.FsObj, *serr.Err) {
	db.DPrintf(db.NAMED, "%v: Create name %q (perm %v lid %v) in dir %v", ctx.ClntId(), name, perm, lid, d)
	start := time.Now()
	defer func(start time.Time) {
		db.DPrintf(db.NAMED, "%v: Create latency %v name %q (perm %v lid %v) in dir %v", ctx.ClntId(), time.Since(start), name, perm, lid, d)
	}(start)
	cid := sp.NoClntId
	if lid.IsLeased() {
		cid = ctx.ClntId()
	}
	pn := d.pn.Copy().Append(name)
	path := newTpath(pn)
	nf, r := fsetcd.NewEtcdFileDir(perm, path, cid, lid)
	if r != nil {
		return nil, serr.NewErrError(r)
	}
	di, c, err := d.fs.Create(&d.Obj.di, pn, path, nf, perm, f, cid, lid)
	d.Obj.fs.PstatUpdate(d.Obj.pn, c)
	if err != nil {
		db.DPrintf(db.NAMED, "Create %v %q err %v", d, name, err)
		return nil, err
	}
	obj := newObjDi(d.fs, pn, *di)
	if obj.di.Perm.IsDir() {
		return newDir(obj), nil
	} else if obj.di.Perm.IsDevice() {
		return newDev(obj), nil
	} else {
		return newFile(obj), nil
	}
}

func (d *Dir) ReadDir(ctx fs.CtxI, cursor int, cnt sp.Tsize) ([]*sp.Tstat, *serr.Err) {
	dir, c, err := d.fs.ReadDir(&d.Obj.di)
	d.Obj.fs.PstatUpdate(d.pn, c)
	if err != nil {
		return nil, err
	}
	if db.WillBePrinted(db.NAMED) {
		db.DPrintf(db.NAMED, "%v: fsetcd.ReadDir %d %v", ctx.ClntId(), cursor, dir)
	}
	len := dir.Ents.Len() - 1 // ignore "."
	if cursor > len {
		return nil, nil
	} else {
		sts := make([]*sp.Tstat, 0, len)
		var r *serr.Err
		dir.Ents.Iter(func(n string, di *fsetcd.DirEntInfo) bool {
			if n != "." {
				db.DPrintf(db.NAMED, "%v: ReadDir iter dir %v di %p di.Nf %p", ctx.ClntId(), dir, di, di.Nf)
				o := newObjDi(d.fs, d.pn.Append(n), *di)
				st, err := o.NewStat()
				if err != nil {
					r = err
					return false
				}
				sts = append(sts, st)
			}
			return true
		})
		if r != nil {
			return nil, r
		}
		return sts[cursor:], nil
	}
}

func (d *Dir) Open(ctx fs.CtxI, m sp.Tmode) (fs.FsObj, *serr.Err) {
	db.DPrintf(db.NAMED, "%p: Open dir %v", d, m)
	return nil, nil
}

func (d *Dir) Close(ctx fs.CtxI, m sp.Tmode) *serr.Err {
	db.DPrintf(db.NAMED, "%p: Close dir %v %v", d, d, m)
	return nil
}

func (d *Dir) Remove(ctx fs.CtxI, name string, f sp.Tfence, del fs.Tdel) *serr.Err {
	db.DPrintf(db.NAMED, "%v: Remove %v name %v", ctx.ClntId(), d, name)
	c, err := d.fs.Remove(&d.Obj.di, name, f, del)
	d.Obj.fs.PstatUpdate(d.pn.Append(name), c)
	return err
}

func (d *Dir) Rename(ctx fs.CtxI, from, to string, f sp.Tfence) *serr.Err {
	db.DPrintf(db.NAMED, "%v: Rename %v: %v -> %v", ctx.ClntId(), d, from, to)
	c, err := d.fs.Rename(&d.Obj.di, from, to, d.pn.Append(to), f)
	d.Obj.fs.PstatUpdate(d.pn.Append(to), c)
	return err
}

func (d *Dir) Renameat(ctx fs.CtxI, from string, od fs.Dir, to string, f sp.Tfence) *serr.Err {
	db.DPrintf(db.NAMED, "%v: Renameat %v: %v %v", ctx.ClntId(), d, from, to)
	dt := od.(*Dir)
	old := d.pn.Append(from)
	new := dt.pn.Append(to)
	c, err := d.fs.Renameat(&d.Obj.di, old, &dt.Obj.di, new, f)
	d.Obj.fs.PstatUpdate(new, c)
	return err
}

// ===== The following functions are needed to make an named dir of type fs.Inode

func (d *Dir) SetMtime(mtime int64) {
	db.DFatalf("Unimplemented")
}

func (d *Dir) Mtime() int64 {
	db.DFatalf("Unimplemented")
	return 0
}

func (d *Dir) Unlink() {
	db.DFatalf("Unimplemented")
}

func (d *Dir) VersionInc() {
	db.DFatalf("Unimplemented")
}

//
// Helpers
//

func RootDir(fs *fsetcd.FsEtcd, realm sp.Trealm) *Dir {
	_, c, err := fs.ReadRootDir()
	fs.PstatUpdate(path.Tpathname{}, c)
	if err != nil && err.IsErrNotfound() { // make root dir
		db.DPrintf(db.NAMED, "fsetcd.ReadDir err %v; make root dir\n", err)
		if err := fs.NewRootDir(); err != nil {
			db.DFatalf("rootDir: newRootDir err %v\n", err)
		}
	} else if err != nil {
		db.DFatalf("rootDir: fsetcd.ReadDir err %v\n", err)
	}
	return newDir(newObjDi(fs, path.Tpathname{}, *fsetcd.NewDirEntInfoDir(fsetcd.ROOT)))
}
