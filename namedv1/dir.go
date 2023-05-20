package namedv1

import (
	"fmt"
	"sync"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/path"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	"sigmaos/sorteddir"
)

type Dir struct {
	*Obj
	sync.Mutex
	dents *sorteddir.SortedDir
}

func (d *Dir) String() string {
	s := d.Obj.String()
	return s + fmt.Sprintf(" dents %v", d.dents)
}

func makeDir(o *Obj) *Dir {
	dir := &Dir{}
	dir.Obj = o
	dir.dents = sorteddir.MkSortedDir()
	return dir
}

func (d *Dir) LookupPath(ctx fs.CtxI, path path.Path) ([]fs.FsObj, fs.FsObj, path.Path, *serr.Err) {
	db.DPrintf(db.NAMEDV1, "%v: Lookup %v o %v\n", ctx, path, d)
	name := path[0]
	if obj, err := getObj(d.pn.Append(name)); err != nil {
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

func (d *Dir) Create(ctx fs.CtxI, name string, perm sp.Tperm, m sp.Tmode) (fs.FsObj, *serr.Err) {
	db.DPrintf(db.NAMEDV1, "Create %v name: %v\n", d, name)
	pn := d.pn.Append(name)
	obj, err := mkObj(pn, perm)
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
	if objs, err := readDir(d.pn); err != nil {
		return nil, err
	} else {
		for _, o := range objs {
			st := o.stat()
			d.dents.Insert(st.Name, st)
		}
	}
	db.DPrintf(db.NAMEDV1, "ReadDir %v\n", d.dents)
	if cursor > d.dents.Len() {
		return nil, nil
	} else {
		// XXX move into sorteddir
		ns := d.dents.Slice(cursor)
		sts := make([]*sp.Stat, len(ns))
		for i, n := range ns {
			e, _ := d.dents.Lookup(n)
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

func (d *Dir) Remove(ctx fs.CtxI, name string) *serr.Err {
	db.DPrintf(db.NAMEDV1, "Delete %v name %v\n", d, name)
	return nil
}

func (d *Dir) Rename(ctx fs.CtxI, from, to string) *serr.Err {
	return serr.MkErr(serr.TErrNotSupported, "Rename")
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
