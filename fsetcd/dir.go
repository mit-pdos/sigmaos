package fsetcd

import (
	"fmt"
	"log"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/sortedmap"
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
	Nf      *EtcdFile
	Path    sp.Tpath
	Perm    sp.Tperm
	ClntId  sp.TclntId
	LeaseId sp.TleaseId
}

func NewDirEntInfo(nf *EtcdFile, p sp.Tpath, perm sp.Tperm, cid sp.TclntId, lid sp.TleaseId) *DirEntInfo {
	return &DirEntInfo{Nf: nf, Path: p, Perm: perm, ClntId: cid, LeaseId: lid}
}

func NewDirEntInfoP(p sp.Tpath, perm sp.Tperm) *DirEntInfo {
	return NewDirEntInfo(nil, p, perm, sp.NoClntId, sp.NoLeaseId)
}

func NewDirEntInfoDir(p sp.Tpath) *DirEntInfo {
	return NewDirEntInfoP(p, sp.DMDIR)
}

func (di DirEntInfo) String() string {
	if di.Nf != nil {
		return fmt.Sprintf("{p %v perm %v cid %v lid %v len %d}", di.Path, di.Perm, di.ClntId, di.LeaseId, len(di.Nf.Data))
	} else {
		return fmt.Sprintf("{p %v perm %v cid %v lid %v}", di.Path, di.Perm, di.ClntId, di.LeaseId)
	}
}

type DirInfo struct {
	Ents *sortedmap.SortedMap[string, *DirEntInfo]
	Perm sp.Tperm
}

func (fse *FsEtcd) isEmpty(dei *DirEntInfo) (bool, *serr.Err) {
	if dei.Perm.IsDir() {
		dir, _, err := fse.readDir(dei, TSTAT_NONE)
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

func (fse *FsEtcd) NewRootDir() *serr.Err {
	nf, r := NewEtcdFileDir(sp.DMDIR, ROOT, sp.NoClntId, sp.NoLeaseId)
	if r != nil {
		db.DPrintf(db.FSETCD, "NewEtcdFileDir err %v", r)
		return serr.NewErrError(r)
	}
	dei := NewDirEntInfo(nf, ROOT, nf.Tperm(), sp.NoClntId, sp.NoLeaseId)
	if err := fse.PutFile(dei, nf, sp.NoFence()); err != nil {
		db.DPrintf(db.FSETCD, "NewRootDir PutFile err %v", err)
		return err
	}
	db.DPrintf(db.FSETCD, "newRoot: PutFile %v\n", nf)
	return nil
}

func (fse *FsEtcd) ReadRootDir() (*DirInfo, *serr.Err) {
	return fse.ReadDir(NewDirEntInfoP(ROOT, sp.DMDIR))
}

func (fse *FsEtcd) Lookup(dei *DirEntInfo, pn path.Tpathname) (*DirEntInfo, *serr.Err) {
	name := pn.Base()
	dir, _, err := fse.readDir(dei, TSTAT_NONE)
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.FSETCD, "Lookup %q %v %v\n", name, dei.Path, dir)
	e, ok := dir.Ents.Lookup(name)
	if ok {
		return e, nil
	}
	return nil, serr.NewErr(serr.TErrNotfound, name)
}

// OEXCL: should only succeed if file doesn't exist
func (fse *FsEtcd) Create(dei *DirEntInfo, pn path.Tpathname, path sp.Tpath, nf *EtcdFile, f sp.Tfence, cid sp.TclntId, lid sp.TleaseId) (*DirEntInfo, *serr.Err) {
	name := pn.Base()
	dir, v, err := fse.readDir(dei, TSTAT_NONE)
	if err != nil {
		return nil, err
	}
	_, ok := dir.Ents.Lookup(name)
	if ok {
		return nil, serr.NewErr(serr.TErrExists, name)
	}
	// Insert name into dir so that fse.create() will write the updated
	// directory to etcd, but undo the Insert if create fails.
	di := NewDirEntInfo(nf, path, nf.Tperm(), cid, lid)
	dir.Ents.Insert(name, di)
	db.DPrintf(db.FSETCD, "Create %q(%v) dir %v %v nf %v\n", name, pn, dir, dei.Path, nf)
	if err := fse.create(dei, dir, v, di, pn); err == nil {
		fse.dc.update(dei.Path, dir)
		return di, nil
	} else {
		db.DPrintf(db.FSETCD, "Create %q dir %v nf %v err %v", name, dir, nf, err)
		dir.Ents.Delete(name)
		return nil, err
	}
}

func (fse *FsEtcd) ReadDir(dei *DirEntInfo) (*DirInfo, *serr.Err) {
	dir, _, err := fse.readDir(dei, TSTAT_STAT)
	if err != nil {
		return nil, err
	}
	return dir, nil
}

// If fsetcd already deleted di because di is a leased file; update
// the on-disk directory to remove the file's entry.
func (fse *FsEtcd) updateLeased(dei *DirEntInfo, dir *DirInfo, v sp.TQversion) *serr.Err {
	if err := fse.updateDir(dei, dir, v); err != nil {
		db.DPrintf(db.FSETCD, "updateLeased %v %v err %v\n", dei, dir, err)
		return err
	}
	fse.dc.update(dei.Path, dir)
	return nil
}

// Remove `name` and delete its directory entry.  To update only the
// directory for a file that etcd already deleted (a leased file), set
// del to false.
func (fse *FsEtcd) Remove(dei *DirEntInfo, name string, f sp.Tfence, del fs.Tdel) *serr.Err {
	dir, v, err := fse.readDir(dei, TSTAT_NONE)
	if err != nil {
		return err
	}
	di, ok := dir.Ents.Lookup(name)
	if !ok {
		return serr.NewErr(serr.TErrNotfound, name)
	}

	db.DPrintf(db.FSETCD, "Remove in %v entry %v %v v %v\n", dir, name, di, v)

	empty, err := fse.isEmpty(di)
	if err != nil {
		return err
	}
	if !empty {
		return serr.NewErr(serr.TErrNotEmpty, name)
	}

	dir.Ents.Delete(name)

	if err := fse.remove(dei, dir, v, di); err != nil {
		db.DPrintf(db.FSETCD, "Remove entry %v %v err %v\n", name, di, err)
		if di.LeaseId.IsLeased() && err.IsErrNotfound() {
			if r := fse.updateLeased(dei, dir, v); r == nil {
				if del == fs.DEL_EXIST {
					return err // return original err
				} else {
					return nil
				}

			} else {
				return r
			}
		}
		dir.Ents.Insert(name, di)
		return err
	}
	fse.dc.update(dei.Path, dir)
	return nil
}

func (fse *FsEtcd) Rename(dei *DirEntInfo, from, to string, new path.Tpathname, f sp.Tfence) *serr.Err {
	dir, v, err := fse.readDir(dei, TSTAT_NONE)
	if err != nil {
		return err
	}
	db.DPrintf(db.FSETCD, "Rename in %v from %v to %v\n", dir, from, to)
	difrom, ok := dir.Ents.Lookup(from)
	if !ok {
		return serr.NewErr(serr.TErrNotfound, from)
	}
	dito, ok := dir.Ents.Lookup(to)
	if ok {
		empty, err := fse.isEmpty(dito)
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
	if err := fse.rename(dei, dir, v, dito, difrom, new); err == nil {
		fse.dc.update(dei.Path, dir)
		return nil
	} else {
		if difrom.LeaseId.IsLeased() && err.IsErrNotfound() {
			if r := fse.updateLeased(dei, dir, v); r == nil {
				return err // return original err
			} else {
				return r
			}
		}
		dir.Ents.Insert(from, difrom)
		dir.Ents.Delete(to)
		return err
	}
}

func (fse *FsEtcd) Renameat(deif *DirEntInfo, from path.Tpathname, deit *DirEntInfo, to path.Tpathname, f sp.Tfence) *serr.Err {
	dirf, vf, err := fse.readDir(deif, TSTAT_NONE)
	if err != nil {
		return err
	}
	dirt, vt, err := fse.readDir(deit, TSTAT_NONE)
	if err != nil {
		return err
	}
	db.DPrintf(db.FSETCD, "Renameat %v dir: %v %v %v %v\n", deif.Path, dirf, dirt, vt, vf)
	difrom, ok := dirf.Ents.Lookup(from.Base())
	if !ok {
		return serr.NewErr(serr.TErrNotfound, from)
	}
	dito, ok := dirt.Ents.Lookup(to.Base())
	if ok {
		empty, err := fse.isEmpty(dito)
		if err != nil {
			return err
		}
		if !empty {
			return serr.NewErr(serr.TErrNotEmpty, to)
		}
	}
	if ok {
		dirt.Ents.Delete(to.Base())
	}
	dirf.Ents.Delete(from.Base())
	dirt.Ents.Insert(to.Base(), difrom)
	if err := fse.renameAt(deif, dirf, vf, deit, dirt, vt, dito, difrom, to); err == nil {
		fse.dc.update(deif.Path, dirf)
		fse.dc.update(deit.Path, dirt)
		return nil
	} else {
		if difrom.LeaseId.IsLeased() && err.IsErrNotfound() {
			if r := fse.updateLeased(deif, dirf, vf); r == nil {
				return err // return original err
			} else {
				return r
			}
		}
		dirf.Ents.Insert(from.Base(), difrom)
		dirt.Ents.Delete(to.Base())
		return err
	}
}

// XXX if ran as test, it cannot fix dirs with expired ephemeral files
func (fse *FsEtcd) Dump(l int, dir *DirInfo, pn path.Tpathname, p sp.Tpath) error {
	s := ""
	for i := 0; i < l*4; i++ {
		s += " "
	}
	dir.Ents.Iter(func(name string, di *DirEntInfo) bool {
		if name != "." {
			fmt.Printf("%v%v %v\n", s, pn.Append(name), di)
			if di.Perm.IsDir() {
				nd, _, err := fse.readDir(di, TSTAT_NONE)
				if err == nil {
					fse.Dump(l+1, nd, pn.Append(name), di.Path)
				} else {
					log.Printf("dumpDir: getObj %q %v\n", name, err)
				}
			}
		}
		return true
	})
	return nil
}
