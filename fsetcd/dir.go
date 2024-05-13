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

func (fs *FsEtcd) isEmpty(di DirEntInfo) (bool, *serr.Err) {
	if di.Perm.IsDir() {
		dir, _, err := fs.readDir(di.Path, TSTAT_NONE)
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
	if err := fs.PutFile(ROOT, nf, sp.NoFence()); err != nil {
		db.DPrintf(db.FSETCD, "NewRootDir PutFile err %v", err)
		return err
	}
	db.DPrintf(db.FSETCD, "newRoot: PutFile %v\n", nf)
	return nil
}

func (fs *FsEtcd) ReadRootDir() (*DirInfo, *serr.Err) {
	return fs.ReadDir(ROOT)
}

func (fs *FsEtcd) Lookup(d sp.Tpath, name string) (DirEntInfo, *serr.Err) {
	dir, _, err := fs.readDir(d, TSTAT_NONE)
	if err != nil {
		return DirEntInfo{}, err
	}
	db.DPrintf(db.FSETCD, "Lookup %q %v %v\n", name, d, dir)
	e, ok := dir.Ents.Lookup(name)
	if ok {
		return e.(DirEntInfo), nil
	}
	return DirEntInfo{}, serr.NewErr(serr.TErrNotfound, name)
}

// OEXCL: should only succeed if file doesn't exist
func (fs *FsEtcd) Create(d sp.Tpath, name string, path sp.Tpath, nf *EtcdFile, f sp.Tfence) (DirEntInfo, *serr.Err) {
	dir, v, err := fs.readDir(d, TSTAT_NONE)
	if err != nil {
		return DirEntInfo{}, err
	}
	_, ok := dir.Ents.Lookup(name)
	if ok {
		return DirEntInfo{}, serr.NewErr(serr.TErrExists, name)
	}
	// Insert name into dir so that fs.create() will write the updated
	// directory to etcd, but undo the Insert if create fails.
	dir.Ents.Insert(name, DirEntInfo{Nf: nf, Path: path, Perm: nf.Tperm()})
	db.DPrintf(db.FSETCD, "Create %q dir %v (%v) nf %v\n", name, dir, d, nf)
	if err := fs.create(d, dir, v, path, nf); err == nil {
		di := DirEntInfo{Nf: nf, Perm: nf.Tperm(), Path: path}
		if nf.Tperm().IsEphemeral() {
			// don't cache directory anymore
			fs.dc.remove(d)
		} else {
			fs.dc.update(d, dir)
		}
		return di, nil
	} else {
		db.DPrintf(db.FSETCD, "Create %q dir %v nf %v err %v", name, dir, nf, err)
		dir.Ents.Delete(name)
		return DirEntInfo{}, err
	}
}

func (fs *FsEtcd) ReadDir(d sp.Tpath) (*DirInfo, *serr.Err) {
	dir, _, err := fs.readDir(d, TSTAT_STAT)
	if err != nil {
		return nil, err
	}
	return dir, nil
}

func (fs *FsEtcd) Remove(d sp.Tpath, name string, f sp.Tfence) *serr.Err {
	dir, v, err := fs.readDir(d, TSTAT_NONE)
	if err != nil {
		return err
	}
	e, ok := dir.Ents.Lookup(name)
	if !ok {
		return serr.NewErr(serr.TErrNotfound, name)
	}

	di := e.(DirEntInfo)
	db.DPrintf(db.FSETCD, "Remove in %v entry %v %v v %v\n", dir, name, di, v)

	empty, err := fs.isEmpty(di)
	if err != nil {
		return err
	}
	if !empty {
		return serr.NewErr(serr.TErrNotEmpty, name)
	}

	dir.Ents.Delete(name)

	if err := fs.remove(d, dir, v, di.Path); err != nil {
		db.DPrintf(db.FSETCD, "Remove entry %v err %v\n", name, err)
		dir.Ents.Insert(name, di)
		return err
	}
	fs.dc.update(d, dir)
	return nil
}

func (fs *FsEtcd) Rename(d sp.Tpath, from, to string, f sp.Tfence) *serr.Err {
	dir, v, err := fs.readDir(d, TSTAT_NONE)
	if err != nil {
		return err
	}
	db.DPrintf(db.FSETCD, "Rename in %v from %v to %v\n", dir, from, to)
	fromi, ok := dir.Ents.Lookup(from)
	if !ok {
		return serr.NewErr(serr.TErrNotfound, from)
	}
	difrom := fromi.(DirEntInfo)
	topath := sp.Tpath(0)
	toi, ok := dir.Ents.Lookup(to)
	if ok {
		di := toi.(DirEntInfo)
		empty, err := fs.isEmpty(di)
		if err != nil {
			return err
		}
		if !empty {
			return serr.NewErr(serr.TErrNotEmpty, to)
		}
		topath = di.Path
	}
	if ok {
		dir.Ents.Delete(to)
	}
	dir.Ents.Delete(from)
	dir.Ents.Insert(to, difrom)
	if err := fs.rename(d, dir, v, topath); err == nil {
		fs.dc.update(d, dir)
		return nil
	} else {
		dir.Ents.Insert(from, difrom)
		dir.Ents.Delete(to)
		return err
	}
}

func (fs *FsEtcd) Renameat(df sp.Tpath, from string, dt sp.Tpath, to string, f sp.Tfence) *serr.Err {
	dirf, vf, err := fs.readDir(df, TSTAT_NONE)
	if err != nil {
		return err
	}
	dirt, vt, err := fs.readDir(dt, TSTAT_NONE)
	if err != nil {
		return err
	}
	db.DPrintf(db.FSETCD, "Renameat %v dir: %v %v %v %v\n", df, dirf, dirt, vt, vf)
	fi, ok := dirf.Ents.Lookup(from)
	if !ok {
		return serr.NewErr(serr.TErrNotfound, from)
	}
	difrom := fi.(DirEntInfo)
	topath := sp.Tpath(0)
	ti, ok := dirt.Ents.Lookup(to)
	if ok {
		di := ti.(DirEntInfo)
		empty, err := fs.isEmpty(di)
		if err != nil {
			return err
		}
		if !empty {
			return serr.NewErr(serr.TErrNotEmpty, to)
		}
		topath = di.Path
	}
	if ok {
		dirt.Ents.Delete(to)
	}
	dirf.Ents.Delete(from)
	dirt.Ents.Insert(to, difrom)
	if err := fs.renameAt(df, dirf, vf, dt, dirt, vt, topath); err == nil {
		fs.dc.update(df, dirf)
		if difrom.Perm.IsEphemeral() {
			// don't cache directory anymore
			fs.dc.remove(dt)
		} else {
			fs.dc.update(dt, dirt)
		}
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
			di := v.(DirEntInfo)
			fmt.Printf("%v%v %v\n", s, pn.Append(name), di)
			if di.Perm.IsDir() {
				nd, _, err := fs.readDir(di.Path, TSTAT_NONE)
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
