package namesrv

import (
	"time"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/fsetcd"
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

func (d *Dir) LookupPath(ctx fs.CtxI, pn path.Path) ([]fs.FsObj, fs.FsObj, path.Path, *serr.Err) {
	s := time.Now()
	db.DPrintf(db.NAMED, "%v: Lookup %v o %v\n", ctx, pn, d)
	name := pn[0]
	di, err := d.fs.Lookup(d.Obj.di.Path, name)
	if err == nil {
		pn1 := d.pn.Copy().Append(name)
		obj := newObjDi(d.fs, pn1, di, d.Obj.di.Path)
		var o fs.FsObj
		if obj.di.Perm.IsDir() {
			o = newDir(obj)
		} else if obj.di.Perm.IsDevice() {
			o = newDev(obj)
		} else {
			o = newFile(obj)
		}
		db.DPrintf(db.WALK_LAT, "Lookup %v %q %v lat %v\n", ctx.ClntId(), name, d, time.Since(s))

		return []fs.FsObj{o}, o, pn[1:], nil
	}
	return nil, nil, pn, err
}

func (d *Dir) Create(ctx fs.CtxI, name string, perm sp.Tperm, m sp.Tmode, lid sp.TleaseId, f sp.Tfence) (fs.FsObj, *serr.Err) {
	db.DPrintf(db.NAMED, "Create %v name: %v perm %v lid %v\n", d, name, perm, lid)
	cid := sp.NoClntId
	if perm.IsEphemeral() {
		cid = ctx.ClntId()
	}
	pn := d.pn.Copy().Append(name)
	path := newTpath(pn)
	nf, r := fsetcd.NewEtcdFileDir(perm, path, cid, lid)
	if r != nil {
		return nil, serr.NewErrError(r)
	}
	di, err := d.fs.Create(d.Obj.di.Path, name, path, nf, f)
	if err != nil {
		db.DPrintf(db.NAMED, "Create %v %q err %v\n", d, name, err)
		return nil, err
	}
	obj := newObjDi(d.fs, pn, di, d.Obj.di.Path)
	if obj.di.Perm.IsDir() {
		return newDir(obj), nil
	} else if obj.di.Perm.IsDevice() {
		return newDev(obj), nil
	} else {
		return newFile(obj), nil
	}
}

func (d *Dir) ReadDir(ctx fs.CtxI, cursor int, cnt sp.Tsize) ([]*sp.Stat, *serr.Err) {
	dir, err := d.fs.ReadDir(d.Obj.di.Path)
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.NAMED, "fsetcd.ReadDir %d %v\n", cursor, dir)
	len := dir.Ents.Len() - 1 // ignore "."
	if cursor > len {
		return nil, nil
	} else {
		sts := make([]*sp.Stat, 0, len)
		var r *serr.Err
		dir.Ents.Iter(func(n string, e interface{}) bool {
			if n != "." {
				di := e.(fsetcd.DirEntInfo)
				o := newObjDi(d.fs, d.pn.Append(n), di, d.Obj.di.Path)
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
	db.DPrintf(db.NAMED, "%p: Open dir %v\n", d, m)
	return nil, nil
}

func (d *Dir) Close(ctx fs.CtxI, m sp.Tmode) *serr.Err {
	db.DPrintf(db.NAMED, "%p: Close dir %v %v\n", d, d, m)
	return nil
}

func (d *Dir) Remove(ctx fs.CtxI, name string, f sp.Tfence) *serr.Err {
	db.DPrintf(db.NAMED, "Remove %v name %v\n", d, name)
	return d.fs.Remove(d.Obj.di.Path, name, f)
}

func (d *Dir) Rename(ctx fs.CtxI, from, to string, f sp.Tfence) *serr.Err {
	db.DPrintf(db.NAMED, "Rename %v: %v %v\n", d, from, to)
	return d.fs.Rename(d.Obj.di.Path, from, to, f)
}

func (d *Dir) Renameat(ctx fs.CtxI, from string, od fs.Dir, to string, f sp.Tfence) *serr.Err {
	db.DPrintf(db.NAMED, "Renameat %v: %v %v\n", d, from, to)
	dt := od.(*Dir)
	return d.fs.Renameat(d.Obj.di.Path, from, dt.Obj.di.Path, to, f)
}

// ===== The following functions are needed to make an named dir of type fs.Inode

func (d *Dir) SetMtime(mtime int64) {
	db.DFatalf("Unimplemented")
}

func (d *Dir) Mtime() int64 {
	db.DFatalf("Unimplemented")
	return 0
}

func (d *Dir) SetParent(di fs.Dir) {
	db.DFatalf("Unimplemented")
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

func rootDir(fs *fsetcd.FsEtcd, realm sp.Trealm) *Dir {
	_, err := fs.ReadRootDir()
	if err != nil && err.IsErrNotfound() { // make root dir
		db.DPrintf(db.NAMED, "fsetcd.ReadDir err %v; make root dir\n", err)
		if err := fs.NewRootDir(); err != nil {
			db.DFatalf("rootDir: newRootDir err %v\n", err)
		}
	} else if err != nil {
		db.DFatalf("rootDir: fsetcd.ReadDir err %v\n", err)
	}
	return newDir(newObjDi(fs, path.Path{},
		fsetcd.DirEntInfo{Perm: sp.DMDIR | 0777, Path: fsetcd.ROOT},
		fsetcd.ROOT))
}
