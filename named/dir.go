package named

import (
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/fsetcd"
	"sigmaos/path"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type Dir struct {
	*Obj
}

func (d *Dir) String() string {
	return d.Obj.String()
}

func makeDir(o *Obj) *Dir {
	dir := &Dir{Obj: o}
	return dir
}

func (d *Dir) LookupPath(ctx fs.CtxI, pn path.Path) ([]fs.FsObj, fs.FsObj, path.Path, *serr.Err) {
	db.DPrintf(db.NAMED, "%v: Lookup %v o %v\n", ctx, pn, d)
	name := pn[0]
	di, err := d.fs.Lookup(d.Obj.di.Path, name)
	if err == nil {
		pn1 := d.pn.Copy().Append(name)
		obj := makeObjDi(d.fs, pn1, di, d.Obj.di.Path)
		var o fs.FsObj
		if obj.di.Perm.IsDir() {
			o = makeDir(obj)
		} else if obj.di.Perm.IsDevice() {
			o = makeDev(obj)
		} else {
			o = makeFile(obj)
		}
		return []fs.FsObj{o}, o, pn[1:], nil
	}
	return nil, nil, pn, err
}

func (d *Dir) Create(ctx fs.CtxI, name string, perm sp.Tperm, m sp.Tmode) (fs.FsObj, *serr.Err) {
	db.DPrintf(db.NAMED, "Create %v name: %v %v\n", d, name, perm)
	cid := sp.NoClntId
	if perm.IsEphemeral() {
		cid = ctx.ClntId()
	}
	pn := d.pn.Copy().Append(name)
	path := mkTpath(pn)
	nf, r := fsetcd.MkEtcdFileDir(perm, path, cid)
	if r != nil {
		return nil, serr.MkErrError(r)
	}
	di, err := d.fs.Create(d.Obj.di.Path, name, path, nf)
	if err != nil {
		return nil, err
	}
	obj := makeObjDi(d.fs, pn, di, d.Obj.di.Path)
	if obj.di.Perm.IsDir() {
		return makeDir(obj), nil
	} else if obj.di.Perm.IsDevice() {
		return makeDev(obj), nil
	} else {
		return makeFile(obj), nil
	}
}

func (d *Dir) ReadDir(ctx fs.CtxI, cursor int, cnt sessp.Tsize, v sp.TQversion) ([]*sp.Stat, *serr.Err) {
	dir, err := d.fs.ReadDir(d.Obj.di.Path)
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.NAMED, "fsetcd.ReadDir %v\n", dir)
	if cursor > dir.Ents.Len() {
		return nil, nil
	} else {
		// XXX move into sorteddir
		ns := dir.Ents.Slice(cursor)
		sts := make([]*sp.Stat, len(ns))
		for i, n := range ns {
			e, _ := dir.Ents.Lookup(n)
			di := e.(fsetcd.DirEntInfo)
			o := makeObjDi(d.fs, d.pn.Append(n), di, d.Obj.di.Path)
			sts[i] = o.stat()
		}
		return sts, nil
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

func (d *Dir) Remove(ctx fs.CtxI, name string) *serr.Err {
	db.DPrintf(db.NAMED, "Remove %v name %v\n", d, name)
	return d.fs.Remove(d.Obj.di.Path, name)
}

func (d *Dir) Rename(ctx fs.CtxI, from, to string) *serr.Err {
	db.DPrintf(db.NAMED, "Rename %v: %v %v\n", d, from, to)
	return d.fs.Rename(d.Obj.di.Path, from, to)
}

func (d *Dir) Renameat(ctx fs.CtxI, from string, od fs.Dir, to string) *serr.Err {
	db.DPrintf(db.NAMED, "Renameat %v: %v %v\n", d, from, to)
	dt := od.(*Dir)
	return d.fs.Renameat(d.Obj.di.Path, from, dt.Obj.di.Path, to)
}

func (d *Dir) WriteDir(ctx fs.CtxI, off sp.Toffset, b []byte, v sp.TQversion) (sessp.Tsize, *serr.Err) {
	return 0, serr.MkErr(serr.TErrIsdir, d)
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

func (d *Dir) Snapshot(fs.SnapshotF) []byte {
	db.DFatalf("Unimplemented")
	return nil
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
		if err := fs.MkRootDir(); err != nil {
			db.DFatalf("rootDir: mkRootDir err %v\n", err)
		}
	} else if err != nil {
		db.DFatalf("rootDir: fsetcd.ReadDir err %v\n", err)
	}
	return makeDir(makeObjDi(fs, path.Path{},
		fsetcd.DirEntInfo{Perm: sp.DMDIR | 0777, Path: fsetcd.ROOT},
		fsetcd.ROOT))
}
