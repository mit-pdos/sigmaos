package named

import (
	"fmt"
	"log"

	"go.etcd.io/etcd/client/v3"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/fsetcd"
	"sigmaos/path"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	"sigmaos/sorteddir"
)

const (
	ROOT sessp.Tpath = 1
)

type Dir struct {
	*Obj
}

func (d *Dir) String() string {
	return d.Obj.String()
}

func makeDir(o *Obj) *Dir {
	dir := &Dir{}
	dir.Obj = o
	return dir
}

func (d *Dir) LookupPath(ctx fs.CtxI, path path.Path) ([]fs.FsObj, fs.FsObj, path.Path, *serr.Err) {
	db.DPrintf(db.NAMED, "%v: Lookup %v o %v\n", ctx, path, d)
	dir, _, err := d.ec.ReadDir(d.Obj.path)
	if err != nil {
		return nil, nil, path, err
	}
	e, ok := lookup(dir, path[0])
	if ok {
		pn := d.pn.Copy().Append(e.Name)
		if obj, err := getObj(d.ec, pn, sessp.Tpath(e.Path), d.Obj.path); err != nil {
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
	return nil, nil, path, serr.MkErr(serr.TErrNotfound, path[0])
}

// XXX hold lock?
func (d *Dir) Create(ctx fs.CtxI, name string, perm sp.Tperm, m sp.Tmode) (fs.FsObj, *serr.Err) {
	db.DPrintf(db.NAMED, "Create %v name: %v %v\n", d, name, perm)
	dir, v, err := d.ec.ReadDir(d.Obj.path)
	if err != nil {
		return nil, err
	}
	_, ok := lookup(dir, name)
	if ok {
		return nil, serr.MkErr(serr.TErrExists, name)
	}
	pn := d.pn.Copy().Append(name)
	path := mkTpath(pn)
	cid := sp.NoClntId
	if perm.IsEphemeral() {
		cid = ctx.ClntId()
	}
	db.DPrintf(db.NAMED, "Create %v in %v dir: %v v %v p %v cid %v\n", name, d, dir, v, path, cid)
	dir.Ents = append(dir.Ents, &fsetcd.DirEnt{Name: name, Path: uint64(path)})
	nf, r := mkNamedFile(perm, path, cid)
	if r != nil {
		return nil, r
	}
	if err := d.ec.Create(pn, d.Obj.path, dir, d.perm, v, path, nf); err != nil {
		return nil, err
	}
	obj := makeObj(d.ec, pn, perm, cid, clientv3.LeaseID(nf.LeaseId), 0, path, d.Obj.path, nil)
	if obj.perm.IsDir() {
		return makeDir(obj), nil
	} else {
		return makeFile(obj), nil
	}
}

func (d *Dir) ReadDir(ctx fs.CtxI, cursor int, cnt sessp.Tsize, v sp.TQversion) ([]*sp.Stat, *serr.Err) {
	dents := sorteddir.MkSortedDir()
	if dir, _, err := d.ec.ReadDir(d.Obj.path); err != nil {
		return nil, err
	} else {
		for _, e := range dir.Ents {
			if e.Name != "." {
				obj, err := getObj(d.ec, d.pn.Append(e.Name), sessp.Tpath(e.Path), d.Obj.path)
				if err != nil {
					db.DPrintf(db.NAMED, "ReadDir: getObj %v %v\n", e.Name, err)
					continue
				}
				dents.Insert(e.Name, obj.stat())
			}
		}
	}
	db.DPrintf(db.NAMED, "fsetcd.ReadDir %v\n", dents)
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
	db.DPrintf(db.NAMED, "%p: Open dir %v\n", d, m)
	return nil, nil
}

func (d *Dir) Close(ctx fs.CtxI, m sp.Tmode) *serr.Err {
	db.DPrintf(db.NAMED, "%p: Close dir %v %v\n", d, d, m)
	return nil
}

func (d *Dir) Remove(ctx fs.CtxI, name string) *serr.Err {
	db.DPrintf(db.NAMED, "Remove %v name %v\n", d, name)
	dir, v, err := d.ec.ReadDir(d.Obj.path)
	if err != nil {
		return err
	}
	db.DPrintf(db.NAMED, "Remove %v dir: %v v %v\n", d, dir, v)
	path, ok := remove(dir, name)
	if !ok {
		return serr.MkErr(serr.TErrNotfound, name)
	}
	obj, err := getObj(d.ec, d.pn.Append(name), path, d.Obj.path)
	if err != nil {
		return serr.MkErr(serr.TErrNotfound, name)
	}
	if isNonemptyDir(obj) {
		return serr.MkErr(serr.TErrNotEmpty, name)
	}
	if err := d.ec.Remove(d.Obj.path, dir, d.perm, v, path); err != nil {
		return err
	}
	return nil
}

func (d *Dir) Rename(ctx fs.CtxI, from, to string) *serr.Err {
	db.DPrintf(db.NAMED, "Rename %v: %v %v\n", d, from, to)
	dir, v, err := d.ec.ReadDir(d.Obj.path)
	if err != nil {
		return err
	}
	db.DPrintf(db.NAMED, "Rename %v dir: %v v %v\n", d, dir, v)
	frompath, ok := remove(dir, from)
	if !ok {
		return serr.MkErr(serr.TErrNotfound, from)
	}
	toent, ok := lookup(dir, to)
	if ok {
		obj, err := getObj(d.ec, d.pn.Append(to), sessp.Tpath(toent.Path), d.Obj.path)
		if err != nil {
			db.DFatalf("Rename: getObj %v %v\n", to, err)
		}
		if isNonemptyDir(obj) {
			return serr.MkErr(serr.TErrNotEmpty, to)
		}
	}
	topath := sessp.Tpath(0)
	if ok {
		topath, ok = remove(dir, to)
		if !ok {
			db.DFatalf("Rename: remove %v not present\n", to)
		}
	}
	dir.Ents = append(dir.Ents, &fsetcd.DirEnt{Name: to, Path: uint64(frompath)})
	return d.ec.Rename(d.Obj.path, dir, d.perm, v, topath)
}

func (d *Dir) Renameat(ctx fs.CtxI, from string, od fs.Dir, to string) *serr.Err {
	db.DPrintf(db.NAMED, "Renameat %v: %v %v\n", d, from, to)
	dirf, vf, err := d.ec.ReadDir(d.Obj.path)
	if err != nil {
		return err
	}
	dt := od.(*Dir)
	dirt, vt, err := d.ec.ReadDir(dt.Obj.path)
	if err != nil {
		return err
	}
	db.DPrintf(db.NAMED, "Renameat %v dir: %v %v %v %v\n", d, dirf, dirt, vt, vf)
	frompath, ok := remove(dirf, from)
	if !ok {
		return serr.MkErr(serr.TErrNotfound, from)
	}
	toent, ok := lookup(dirt, to)
	if ok {
		obj, err := getObj(d.ec, dt.pn.Append(to), sessp.Tpath(toent.Path), dt.Obj.path)
		if err != nil {
			db.DFatalf("Renameat: getObj %v %v\n", to, err)
		}
		if isNonemptyDir(obj) {
			return serr.MkErr(serr.TErrNotEmpty, to)
		}
	}
	topath := sessp.Tpath(0)
	if ok {
		topath, ok = remove(dirt, to)
		if !ok {
			db.DFatalf("Renameat: remove %v not present\n", to)
		}
	}
	dirt.Ents = append(dirt.Ents, &fsetcd.DirEnt{Name: to, Path: uint64(frompath)})
	return d.ec.RenameAt(d.Obj.path, dirf, d.perm, vf, dt.Obj.path, dirt, dt.perm, vt, topath)
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

func remove(dir *fsetcd.NamedDir, name string) (sessp.Tpath, bool) {
	for i, e := range dir.Ents {
		if e.Name == name {
			p := e.Path
			dir.Ents = append(dir.Ents[:i], dir.Ents[i+1:]...)
			return sessp.Tpath(p), true
		}
	}
	return 0, false
}

func lookup(dir *fsetcd.NamedDir, name string) (*fsetcd.DirEnt, bool) {
	for _, e := range dir.Ents {
		if e.Name == name {
			return e, true
		}
	}
	return nil, false
}

func isNonemptyDir(obj *Obj) bool {
	if obj.perm.IsDir() {
		if dir, err := fsetcd.UnmarshalDir(obj.data); err != nil {
			db.DFatalf("Remove: unmarshalDir %v err %v\n", obj.pn, err)
		} else if len(dir.Ents) > 1 {
			return true
		}
	}
	return false
}

func rootDir(ec *fsetcd.EtcdClnt, realm sp.Trealm) *Dir {
	_, _, err := ec.ReadDir(ROOT)
	if err != nil && err.IsErrNotfound() { // make root dir
		db.DPrintf(db.NAMED, "fsetcd.ReadDir err %v; make root dir\n", err)
		if err := mkRootDir(ec); err != nil {
			db.DFatalf("rootDir: mkRootDir err %v\n", err)
		}
	} else if err != nil {
		db.DFatalf("rootDir: fsetcd.ReadDir err %v\n", err)
	}
	return makeDir(makeObj(ec, path.Path{}, sp.DMDIR|0777, sp.NoClntId, clientv3.NoLease, 0, ROOT, ROOT, nil))
}

func mkRootDir(ec *fsetcd.EtcdClnt) *serr.Err {
	nf, r := mkNamedFile(sp.DMDIR, ROOT, sp.NoClntId)
	if r != nil {
		return r
	}
	if err := ec.PutFile(ROOT, nf); err != nil {
		return err
	}
	db.DPrintf(db.NAMED, "mkRoot: PutFile %v\n", nf)
	return nil
}

func Dump(l int, ec *fsetcd.EtcdClnt, dir *fsetcd.NamedDir, pn path.Path, p sessp.Tpath) error {
	s := ""
	for i := 0; i < l*4; i++ {
		s += " "
	}
	for _, e := range dir.Ents {
		if e.Name != "." {
			o, err := getObj(ec, pn.Append(e.Name), sessp.Tpath(e.Path), p)
			if err != nil {
				log.Printf("dumpDir: getObj %v %v\n", e.Name, err)
				continue
			}
			fmt.Printf("%v%v\n", s, o)
			if o.perm.IsDir() {
				nd, _, err := ec.ReadDir(o.path)
				if err != nil {
					log.Printf("dumpDir: getObj %v %v\n", e.Name, err)
					continue
				}
				Dump(l+1, ec, nd, pn.Append(e.Name), o.path)
			}
		}
	}
	return nil
}
