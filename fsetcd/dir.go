package fsetcd

import (
	"fmt"
	"log"

	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/sorteddir"
)

// This file implements directory operations on top of etcd.  It
// assumes the caller (protsrv) has read/write locks for the
// directories involved in the operation.  Directory entries are a
// (name, etcd key) tuple.  To implement directory operations
// atomically with respect to crashes (e.g., updating the directory
// and creating a file) fsetcd uses etcd's transaction API.

const (
	ROOT sp.Tpath = 1
)

type DirEntInfo struct {
	Nf   *EtcdFile
	Path sp.Tpath
	Perm sp.Tperm
}

func newDirEntInfo(nf *EtcdFile, p sp.Tpath, perm sp.Tperm) *DirEntInfo {
	return &DirEntInfo{nf, p, perm}
}

func newDirEntInfoP(p sp.Tpath, perm sp.Tperm) *DirEntInfo {
	return &DirEntInfo{nil, p, perm}
}

func NewDirEntInfoDir(p sp.Tpath) *DirEntInfo {
	return &DirEntInfo{nil, p, sp.DMDIR}
}

func (di DirEntInfo) String() string {
	if di.Nf != nil {
		return fmt.Sprintf("{p %v perm %v cid %v lid %v len %d}", di.Path, di.Perm, di.Nf.TclntId(), di.Nf.TleaseId(), len(di.Nf.Data))
	} else {
		return fmt.Sprintf("{p %v perm %v}", di.Path, di.Perm)
	}
}

type DirInfo struct {
	Ents *sorteddir.SortedDir
	Perm sp.Tperm
}

func (di *DirInfo) find(del sp.Tpath) (string, bool) {
	for _, n := range di.Ents.Slice(0) {
		e, ok := di.Ents.Lookup(n)
		if ok {
			dei := e.(*DirEntInfo)
			if dei.Path == del {
				db.DPrintf(db.FSETCD, "expire %q %v\n", n, del)
				return n, true
			}
		}
	}
	return "", false
}

func (fs *FsEtcd) isEmpty(dei *DirEntInfo) (bool, *serr.Err) {
	if dei.Perm.IsDir() {
		dir, _, err := fs.readDir(dei, TSTAT_NONE)
		if err != nil {
			return false, err
		}
		if dir.Ents.Len() <= 1 { // don't count "."
			return true, nil
		} else {
			return false, nil
		}
	} else {
		return true, nil
	}
}

func (fs *FsEtcd) NewRootDir() *serr.Err {
	nf, r := NewEtcdFileDir(sp.DMDIR, ROOT, sp.NoClntId, sp.NoLeaseId)
	if r != nil {
		db.DPrintf(db.FSETCD, "NewEtcdFileDir err %v", r)
		return serr.NewErrError(r)
	}
	dei := newDirEntInfo(nf, ROOT, nf.Tperm())
	if err := fs.PutFile(dei, nf, sp.NoFence()); err != nil {
		db.DPrintf(db.FSETCD, "NewRootDir PutFile err %v", err)
		return err
	}
	db.DPrintf(db.FSETCD, "newRoot: PutFile %v\n", nf)
	return nil
}

func (fs *FsEtcd) ReadRootDir() (*DirInfo, *serr.Err) {
	return fs.ReadDir(newDirEntInfoP(ROOT, sp.DMDIR))
}

func (fs *FsEtcd) Lookup(dei *DirEntInfo, name string) (*DirEntInfo, *serr.Err) {
	dir, _, err := fs.readDir(dei, TSTAT_NONE)
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.FSETCD, "Lookup %q %v %v\n", name, dei.Path, dir)
	e, ok := dir.Ents.Lookup(name)
	if ok {
		return e.(*DirEntInfo), nil
	}
	return nil, serr.NewErr(serr.TErrNotfound, name)
}

// OEXCL: should only succeed if file doesn't exist
func (fs *FsEtcd) Create(dei *DirEntInfo, name string, path sp.Tpath, nf *EtcdFile, f sp.Tfence) (*DirEntInfo, *serr.Err) {
	dir, v, err := fs.readDir(dei, TSTAT_NONE)
	if err != nil {
		return nil, err
	}
	_, ok := dir.Ents.Lookup(name)
	if ok {
		return nil, serr.NewErr(serr.TErrExists, name)
	}
	// Insert name into dir so that fs.create() will write the updated
	// directory to etcd, but undo the Insert if create fails.
	di := &DirEntInfo{Nf: nf, Path: path, Perm: nf.Tperm()}
	dir.Ents.Insert(name, di)
	db.DPrintf(db.FSETCD, "Create %q dir %v (%v) nf %v\n", name, dir, dei.Path, nf)
	if err := fs.create(dei, dir, v, di); err == nil {
		fs.dc.update(dei.Path, dir)
		return di, nil
	} else {
		db.DPrintf(db.FSETCD, "Create %q dir %v nf %v err %v", name, dir, nf, err)
		dir.Ents.Delete(name)
		return nil, err
	}
}

func (fs *FsEtcd) ReadDir(dei *DirEntInfo) (*DirInfo, *serr.Err) {
	dir, _, err := fs.readDir(dei, TSTAT_STAT)
	if err != nil {
		return nil, err
	}
	return dir, nil
}

// Remove `name` and delete its directory entry.  To update only the
// directory for a file that etcd already deleted (a leased file), set
// del to false.
func (fs *FsEtcd) Remove(dei *DirEntInfo, name string, f sp.Tfence, del bool) *serr.Err {
	dir, v, err := fs.readDir(dei, TSTAT_NONE)
	if err != nil {
		return err
	}
	e, ok := dir.Ents.Lookup(name)
	if !ok {
		return serr.NewErr(serr.TErrNotfound, name)
	}

	di := e.(*DirEntInfo)
	db.DPrintf(db.FSETCD, "Remove in %v entry %v %v v %v d %t\n", dir, name, di, v, del)

	empty, err := fs.isEmpty(di)
	if err != nil {
		return err
	}
	if !empty {
		return serr.NewErr(serr.TErrNotEmpty, name)
	}

	dir.Ents.Delete(name)

	if del {
		if err := fs.remove(dei, dir, v, di); err != nil {
			db.DPrintf(db.FSETCD, "Remove entry %v err %v\n", name, err)
			if !err.IsErrNotfound() {
				dir.Ents.Insert(name, di)
				return err
			}
			// fsetcd may have deleted di already because di is ephemeral
		}
	} else {
		if err := fs.updateDir(dei, dir, v); err != nil {
			db.DPrintf(db.FSETCD, "Remove updateDir %v %q err %v\n", dir, name, err)
			return err
		}
	}

	fs.dc.update(dei.Path, dir)
	return nil
}

func (fs *FsEtcd) Rename(dei *DirEntInfo, from, to string, f sp.Tfence) *serr.Err {
	dir, v, err := fs.readDir(dei, TSTAT_NONE)
	if err != nil {
		return err
	}
	db.DPrintf(db.FSETCD, "Rename in %v from %v to %v\n", dir, from, to)
	fromi, ok := dir.Ents.Lookup(from)
	if !ok {
		return serr.NewErr(serr.TErrNotfound, from)
	}
	difrom := fromi.(*DirEntInfo)
	var di *DirEntInfo
	toi, ok := dir.Ents.Lookup(to)
	if ok {
		di = toi.(*DirEntInfo)
		empty, err := fs.isEmpty(di)
		if err != nil {
			return err
		}
		if !empty {
			return serr.NewErr(serr.TErrNotEmpty, to)
		}
	}
	if ok {
		dir.Ents.Delete(to)
	}
	dir.Ents.Delete(from)
	dir.Ents.Insert(to, difrom)
	if err := fs.rename(dei, dir, v, di); err == nil {
		fs.dc.update(dei.Path, dir)
		return nil
	} else {
		dir.Ents.Insert(from, difrom)
		dir.Ents.Delete(to)
		return err
	}
}

func (fs *FsEtcd) Renameat(deif *DirEntInfo, from string, deit *DirEntInfo, to string, f sp.Tfence) *serr.Err {
	dirf, vf, err := fs.readDir(deif, TSTAT_NONE)
	if err != nil {
		return err
	}
	dirt, vt, err := fs.readDir(deit, TSTAT_NONE)
	if err != nil {
		return err
	}
	db.DPrintf(db.FSETCD, "Renameat %v dir: %v %v %v %v\n", deif.Path, dirf, dirt, vt, vf)
	fi, ok := dirf.Ents.Lookup(from)
	if !ok {
		return serr.NewErr(serr.TErrNotfound, from)
	}
	difrom := fi.(*DirEntInfo)
	var dito *DirEntInfo
	ti, ok := dirt.Ents.Lookup(to)
	if ok {
		dito = ti.(*DirEntInfo)
		empty, err := fs.isEmpty(dito)
		if err != nil {
			return err
		}
		if !empty {
			return serr.NewErr(serr.TErrNotEmpty, to)
		}
	}
	if ok {
		dirt.Ents.Delete(to)
	}
	dirf.Ents.Delete(from)
	dirt.Ents.Insert(to, difrom)
	if err := fs.renameAt(deif, dirf, vf, deit, dirt, vt, dito); err == nil {
		fs.dc.update(deif.Path, dirf)
		fs.dc.update(deit.Path, dirt)
		return nil
	} else {
		dirf.Ents.Insert(from, difrom)
		dirt.Ents.Delete(to)
		return err
	}
}

func (fs *FsEtcd) Dump(l int, dir *DirInfo, pn path.Path, p sp.Tpath) error {
	s := ""
	for i := 0; i < l*4; i++ {
		s += " "
	}
	dir.Ents.Iter(func(name string, v interface{}) bool {
		if name != "." {
			di := v.(*DirEntInfo)
			fmt.Printf("%v%v %v\n", s, pn.Append(name), di)
			if di.Perm.IsDir() {
				nd, _, err := fs.readDir(di, TSTAT_NONE)
				if err == nil {
					fs.Dump(l+1, nd, pn.Append(name), di.Path)
				} else {
					log.Printf("dumpDir: getObj %v %v\n", name, err)
				}
			}
		}
		return true
	})
	return nil
}
