package namedv1

import (
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/path"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	"sigmaos/sorteddir"
)

const ROOT sessp.Tpath = 1

type Dir struct {
	*Obj
}

func (d *Dir) String() string {
	return d.Obj.String()
}

func rootDir() *Dir {
	_, _, err := readDir(sessp.Tpath(1))
	if err != nil && err.IsErrNotfound() { // make root dir
		db.DPrintf(db.NAMEDV1, "readDir err %v; make root dir\n", err)
		if err := mkRootDir(); err != nil {
			db.DFatalf("rootDir: mkRootDir err %v\n", err)
		}
	} else if err != nil {
		db.DFatalf("rootDir: readDir err %v\n", err)
	}
	return makeDir(makeObj(path.Path{}, sp.DMDIR, 0, ROOT, ROOT, nil))
}

func makeDir(o *Obj) *Dir {
	dir := &Dir{}
	dir.Obj = o
	return dir
}

func (d *Dir) LookupPath(ctx fs.CtxI, path path.Path) ([]fs.FsObj, fs.FsObj, path.Path, *serr.Err) {
	db.DPrintf(db.NAMEDV1, "%v: Lookup %v o %v\n", ctx, path, d)
	dir, _, err := readDir(d.Obj.path)
	if err != nil {
		return nil, nil, path, err
	}
	for _, e := range dir.Ents {
		if e.Name == path[0] {
			pn := d.pn.Copy().Append(e.Name)
			if obj, err := getObj(pn, sessp.Tpath(e.Path), d.Obj.path); err != nil {
				return nil, nil, path, err
			} else {
				var o fs.FsObj
				if obj.perm.IsDir() {
					o = makeDir(obj)
				} else {
					o = makeFile(obj)
				}
				return []fs.FsObj{o}, o, path[1:], nil
			}
		}
	}
	return nil, nil, path, serr.MkErr(serr.TErrNotfound, path[0])
}

// XXX hold lock?
func (d *Dir) Create(ctx fs.CtxI, name string, perm sp.Tperm, m sp.Tmode) (fs.FsObj, *serr.Err) {
	db.DPrintf(db.NAMEDV1, "Create %v name: %v %v\n", d, name, perm)
	dir, v, err := readDir(d.Obj.path)
	if err != nil {
		return nil, err
	}
	pn := d.pn.Copy().Append(name)
	path := mkTpath(pn)
	db.DPrintf(db.NAMEDV1, "Create %v dir: %v v %v p %v\n", d, dir, v, path)
	dir.Ents = append(dir.Ents, &DirEnt{Name: name, Path: uint64(path)})
	obj, err := addObj(pn, d.Obj.path, dir, v, path, perm)
	if err != nil {
		return nil, err
	}
	if obj.perm.IsDir() {
		return makeDir(obj), nil
	} else {
		return makeFile(obj), nil
	}
}

func (d *Dir) ReadDir(ctx fs.CtxI, cursor int, cnt sessp.Tsize, v sp.TQversion) ([]*sp.Stat, *serr.Err) {
	dents := sorteddir.MkSortedDir()
	if dir, _, err := readDir(d.Obj.path); err != nil {
		return nil, err
	} else {
		for _, e := range dir.Ents {
			st := &sp.Stat{Name: e.Name}
			dents.Insert(st.Name, st)
		}
	}
	db.DPrintf(db.NAMEDV1, "ReadDir %v\n", dents)
	if cursor > dents.Len() {
		return nil, nil
	} else {
		// XXX move into sorteddir
		ns := dents.Slice(cursor)
		sts := make([]*sp.Stat, len(ns))
		for i, n := range ns {
			e, _ := dents.Lookup(n)
			sts[i] = e.(*sp.Stat)
		}
		return sts, nil
	}
}

func (d *Dir) Open(ctx fs.CtxI, m sp.Tmode) (fs.FsObj, *serr.Err) {
	db.DPrintf(db.NAMEDV1, "%p: Open dir %v\n", d, m)
	return nil, nil
}

func (d *Dir) Close(ctx fs.CtxI, m sp.Tmode) *serr.Err {
	db.DPrintf(db.NAMEDV1, "%p: Close dir %v\n", d, m)
	return nil
}

func remove(dir *NamedDir, name string) (sessp.Tpath, bool) {
	for i, e := range dir.Ents {
		if e.Name == name {
			p := e.Path
			dir.Ents = append(dir.Ents[:i], dir.Ents[i+1:]...)
			return sessp.Tpath(p), true
		}
	}
	return 0, false
}

func (d *Dir) Remove(ctx fs.CtxI, name string) *serr.Err {
	db.DPrintf(db.NAMEDV1, "Remove %v name %v\n", d, name)
	dir, v, err := readDir(d.Obj.path)
	if err != nil {
		return err
	}
	db.DPrintf(db.NAMEDV1, "Remove %v dir: %v v %v\n", d, dir, v)
	path, ok := remove(dir, name)
	if !ok {
		return serr.MkErr(serr.TErrNotfound, name)
	}
	obj, err := getObj(d.pn.Append(name), path, d.Obj.path)
	if err != nil {
		return serr.MkErr(serr.TErrNotfound, name)
	}
	if obj.perm.IsDir() {
		if dir, err := unmarshalDir(obj.data); err != nil {
			db.DFatalf("Remove: unmarshalDir %v err %v\n", name, err)
		} else if len(dir.Ents) > 0 {
			return serr.MkErr(serr.TErrNotEmpty, name)
		}
	}
	if err := rmObj(d.Obj.path, dir, v, path); err != nil {
		return err
	}
	return nil
}

// XXX check if from and to are files in d
func (d *Dir) Rename(ctx fs.CtxI, from, to string) *serr.Err {
	db.DPrintf(db.NAMEDV1, "Rename %v: %v %v\n", d, from, to)
	f := d.pn.Copy().Append(from)
	t := d.pn.Copy().Append(to)
	return mvObj(f, t)
}

func (d *Dir) Renameat(ctx fs.CtxI, from string, od fs.Dir, to string) *serr.Err {
	return serr.MkErr(serr.TErrNotSupported, "Renameat")
}

func (d *Dir) WriteDir(ctx fs.CtxI, off sp.Toffset, b []byte, v sp.TQversion) (sessp.Tsize, *serr.Err) {
	return 0, serr.MkErr(serr.TErrIsdir, d)
}

// ===== The following functions are needed to make an s3 dir of type fs.Inode

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
