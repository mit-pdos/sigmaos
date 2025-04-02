package fsetcd

import (
	"fmt"
	"log"
	"time"

	"sigmaos/api/fs"
	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv/stats"
	"sigmaos/util/sortedmapv1"
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

func NewDirEntInfoNf(nf *EtcdFile, p sp.Tpath, perm sp.Tperm, cid sp.TclntId, lid sp.TleaseId) *DirEntInfo {
	return &DirEntInfo{
		Nf:      nf,
		Path:    p,
		Perm:    perm,
		ClntId:  cid,
		LeaseId: lid,
	}
}

func NewDirEntInfo(p sp.Tpath, perm sp.Tperm, cid sp.TclntId, lid sp.TleaseId) *DirEntInfo {
	return NewDirEntInfoNf(nil, p, perm, cid, lid)
}

func NewDirEntInfoDir(p sp.Tpath) *DirEntInfo {
	return NewDirEntInfo(p, sp.DMDIR|0777, sp.NoClntId, sp.NoLeaseId)
}

func (di *DirEntInfo) String() string {
	if di.Nf != nil {
		return fmt.Sprintf("&{p %v perm %v cid %v lid %v len %d}", di.Path, di.Perm, di.ClntId, di.LeaseId, len(di.Nf.Data))
	} else {
		return fmt.Sprintf("&{p %v perm %v cid %v lid %v}", di.Path, di.Perm, di.ClntId, di.LeaseId)
	}
}

type DirInfo struct {
	Ents *sortedmapv1.SortedMap[string, *DirEntInfo]
}

func (di *DirInfo) String() string {
	return fmt.Sprintf("DI %p len %d", di, di.Ents.Len())
}

func (fse *FsEtcd) isEmpty(dei *DirEntInfo) (bool, stats.Tcounter, *serr.Err) {
	if dei.Perm.IsDir() {
		dir, _, nops, err := fse.readDir(dei, TSTAT_NONE)
		if err != nil {
			return false, nops, err
		}
		if dir.Ents.Len() <= 1 { // don't count "."
			return true, nops, nil
		} else {
			return false, nops, nil
		}
	} else {
		return true, stats.NewCounter(0), nil
	}
}

func (fse *FsEtcd) NewRootDir() *serr.Err {
	nf, r := NewEtcdFileDir(sp.DMDIR, ROOT, sp.NoClntId, sp.NoLeaseId)
	if r != nil {
		db.DPrintf(db.FSETCD, "NewEtcdFileDir err %v", r)
		return serr.NewErrError(r)
	}
	dei := NewDirEntInfoNf(nf, ROOT, sp.DMDIR, sp.NoClntId, sp.NoLeaseId)
	if _, err := fse.PutFile(dei, nf, sp.NoFence()); err != nil {
		db.DPrintf(db.FSETCD, "NewRootDir PutFile err %v", err)
		return err
	}
	db.DPrintf(db.FSETCD, "newRoot: PutFile %v\n", nf)
	return nil
}

func (fse *FsEtcd) ReadRootDir() (*DirInfo, stats.Tcounter, *serr.Err) {
	return fse.ReadDir(NewDirEntInfoDir(ROOT))
}

func (fse *FsEtcd) Lookup(dei *DirEntInfo, pn path.Tpathname) (*DirEntInfo, *serr.Err) {
	name := pn.Base()
	start := time.Now()
	dir, _, nops, err := fse.readDir(dei, TSTAT_NONE)
	if err != nil {
		fse.PstatUpdate(pn, nops)
		return nil, err
	}
	db.DPrintf(db.FSETCD, "Lookup %q %v %v", name, dei.Path, dir)
	e, ok := dir.Ents.Lookup(name)
	if ok {
		db.DPrintf(db.FSETCD, "Lookup ok %q %v %v", name, dei.Path, dir)
		return e, nil
	}
	db.DPrintf(db.FSETCD, "Lookup Notfound %q %v %v", name, dei.Path, dir)
	db.DPrintf(db.FSETCD_LAT, "Lookup %v %v lat %v", name, dei.Path, time.Since(start))
	return nil, serr.NewErr(serr.TErrNotfound, name)
}

// OEXCL: should only succeed if file doesn't exist
func (fse *FsEtcd) Create(dei *DirEntInfo, pn path.Tpathname, path sp.Tpath, nf *EtcdFile, perm sp.Tperm, f sp.Tfence, cid sp.TclntId, lid sp.TleaseId) (*DirEntInfo, stats.Tcounter, *serr.Err) {
	name := pn.Base()
	start := time.Now()
	db.DPrintf(db.FSETCD, "%v: Create start pn %v lid %v", cid, pn, lid)
	dir, v, nops, err := fse.readDir(dei, TSTAT_NONE)
	if err != nil {
		return nil, stats.NewCounter(0), err
	}
	db.DPrintf(db.FSETCD, "%v: Create readDir pn %v lid %v lat %v", cid, pn, lid, time.Since(start))
	start = time.Now()
	_, ok := dir.Ents.Lookup(name)
	if ok {
		return nil, nops, serr.NewErr(serr.TErrExists, name)
	}
	db.DPrintf(db.FSETCD, "%v: Create Lookup pn %v lid %v lat %v", cid, pn, lid, time.Since(start))
	start = time.Now()
	// Insert name into dir so that fse.create() will write the updated
	// directory to etcd, but undo the Insert if create fails.
	di := NewDirEntInfoNf(nf, path, perm, cid, lid)
	dir.Ents.Insert(name, di)
	db.DPrintf(db.FSETCD, "%v: Create Insert pn %v lid %v lat %v", cid, pn, lid, time.Since(start))
	start = time.Now()
	db.DPrintf(db.FSETCD, "Create %q(%v) di %v f %v\n", name, pn, di, f)
	if nops1, err := fse.create(dei, dir, v, di, pn, f); err == nil {
		stats.Add(&nops, nops1)
		db.DPrintf(db.FSETCD, "%v: Create create pn %v lid %v lat %v", cid, pn, lid, time.Since(start))
		start = time.Now()
		fse.dc.update(dei.Path, dir)
		db.DPrintf(db.FSETCD, "%v: Create dc.update pn %v lid %v lat %v", cid, pn, lid, time.Since(start))
		return di, nops, nil
	} else {
		db.DPrintf(db.FSETCD, "Create %q di %v err %v", name, di, err)
		stats.Add(&nops, nops1)
		dir.Ents.Delete(name)
		return nil, nops, err
	}
}

func (fse *FsEtcd) ReadDir(dei *DirEntInfo) (*DirInfo, stats.Tcounter, *serr.Err) {
	dir, _, nops, err := fse.readDir(dei, TSTAT_STAT)
	if err != nil {
		return nil, nops, err
	}
	return dir, nops, nil
}

// If fsetcd already deleted di because di is a leased file; update
// the on-disk directory to remove the file's entry.
func (fse *FsEtcd) updateLeased(dei *DirEntInfo, dir *DirInfo, v sp.TQversion) (stats.Tcounter, *serr.Err) {
	if c, err := fse.updateDir(dei, dir, v); err != nil {
		db.DPrintf(db.FSETCD, "updateLeased %v %v err %v\n", dei, dir, err)
		return c, err
	} else {
		fse.dc.update(dei.Path, dir)
		return c, nil
	}
}

// Remove `name` and delete its directory entry.  To update only the
// directory for a file that etcd already deleted (a leased file), set
// del to false.
func (fse *FsEtcd) Remove(dei *DirEntInfo, name string, f sp.Tfence, del fs.Tdel) (stats.Tcounter, *serr.Err) {
	dir, v, nops, err := fse.readDir(dei, TSTAT_NONE)
	if err != nil {
		return nops, err
	}
	di, ok := dir.Ents.Lookup(name)
	if !ok {
		return nops, serr.NewErr(serr.TErrNotfound, name)
	}

	db.DPrintf(db.FSETCD, "Remove in %v entry %v %v v %v\n", dir, name, di, v)

	empty, nops1, err := fse.isEmpty(di)
	stats.Add(&nops, nops1)
	if err != nil {
		return nops, err
	}
	if !empty {
		return nops, serr.NewErr(serr.TErrNotEmpty, name)
	}

	dir.Ents.Delete(name)
	nops1, err = fse.remove(dei, dir, v, di, f)
	stats.Add(&nops, nops1)
	if err != nil {
		db.DPrintf(db.FSETCD, "Remove entry %v %v err %v\n", name, di, err)
		if di.LeaseId.IsLeased() && err.IsErrNotfound() {
			if nops1, r := fse.updateLeased(dei, dir, v); r == nil {
				stats.Add(&nops, nops1)
				if del == fs.DEL_EXIST {
					return nops, err // return original err
				} else {
					return nops, nil
				}

			} else {
				return nops, r
			}
		}
		dir.Ents.Insert(name, di)
		return nops, err
	}
	fse.dc.update(dei.Path, dir)
	return nops, nil
}

func (fse *FsEtcd) Rename(dei *DirEntInfo, from, to string, new path.Tpathname, f sp.Tfence) (stats.Tcounter, *serr.Err) {
	dir, v, nops, err := fse.readDir(dei, TSTAT_NONE)
	if err != nil {
		return nops, err
	}
	db.DPrintf(db.FSETCD, "Rename in %v from %v to %v\n", dir, from, to)
	difrom, ok := dir.Ents.Lookup(from)
	if !ok {
		return nops, serr.NewErr(serr.TErrNotfound, from)
	}
	dito, ok := dir.Ents.Lookup(to)
	if ok {
		empty, nops1, err := fse.isEmpty(dito)
		stats.Add(&nops, nops1)
		if err != nil {
			return nops, err
		}
		if !empty {
			return nops, serr.NewErr(serr.TErrNotEmpty, to)
		}
	}
	if ok {
		dir.Ents.Delete(to)
	}

	dir.Ents.Delete(from)
	dir.Ents.Insert(to, difrom)
	if nops1, err := fse.rename(dei, dir, v, dito, difrom, new, f); err == nil {
		stats.Add(&nops, nops1)
		fse.dc.update(dei.Path, dir)
		return nops, nil
	} else {
		stats.Add(&nops, nops1)
		if difrom.LeaseId.IsLeased() && err.IsErrNotfound() {
			if nops1, r := fse.updateLeased(dei, dir, v); r == nil {
				stats.Add(&nops, nops1)
				return nops, err // return original err
			} else {
				stats.Add(&nops, nops1)
				return nops, r
			}
		}
		dir.Ents.Insert(from, difrom)
		dir.Ents.Delete(to)
		return nops, err
	}
}

func (fse *FsEtcd) Renameat(deif *DirEntInfo, from path.Tpathname, deit *DirEntInfo, to path.Tpathname, f sp.Tfence) (stats.Tcounter, *serr.Err) {
	dirf, vf, nops, err := fse.readDir(deif, TSTAT_NONE)
	if err != nil {
		return nops, err
	}
	dirt, vt, nops1, err := fse.readDir(deit, TSTAT_NONE)
	stats.Add(&nops, nops1)
	if err != nil {
		return nops, err
	}
	db.DPrintf(db.FSETCD, "Renameat %v -> %v p %v dir: %v %v %v %v", from, to, deif.Path, dirf, dirt, vf, vt)
	difrom, ok := dirf.Ents.Lookup(from.Base())
	if !ok {
		return nops, serr.NewErr(serr.TErrNotfound, from)
	}
	db.DPrintf(db.FSETCD, "Renameat %v dir: %v %v %v %v difrom %p difrom.Nf %v", deif.Path, dirf, dirt, vf, vt, difrom, difrom.Nf)
	dito, ok := dirt.Ents.Lookup(to.Base())
	if ok {
		empty, nops1, err := fse.isEmpty(dito)
		stats.Add(&nops, nops1)
		if err != nil {
			return nops, err
		}
		if !empty {
			return nops, serr.NewErr(serr.TErrNotEmpty, to)
		}
	}
	if ok {
		dirt.Ents.Delete(to.Base())
	}
	dirf.Ents.Delete(from.Base())
	dirt.Ents.Insert(to.Base(), difrom)
	nops1, err = fse.renameAt(deif, dirf, vf, deit, dirt, vt, dito, difrom, to)
	stats.Add(&nops, nops1)
	if err == nil {
		fse.dc.update(deif.Path, dirf)
		fse.dc.update(deit.Path, dirt)
		// If the source directory was cached with TSTAT_NONE, note that the
		// destination directory will have to be re-stat-ed now.
		dcf, ok := fse.dc.lookup(deif.Path)
		if ok {
			dct, ok := fse.dc.lookup(deit.Path)
			if ok {
				if dcf.stat == TSTAT_NONE {
					dct.stat = TSTAT_NONE
				}
			}
		}
		return nops, nil
	} else {
		if difrom.LeaseId.IsLeased() && err.IsErrNotfound() {
			stats.Inc(&nops, 1)
			nops1, r := fse.updateLeased(deif, dirf, vf)
			stats.Add(&nops, nops1)
			if r == nil {
				return nops, err // return original err
			} else {
				return nops, r
			}
		}
		dirf.Ents.Insert(from.Base(), difrom)
		dirt.Ents.Delete(to.Base())
		return nops, err
	}
}

// If run from test, it will not fix dirs with expired leased files
func (fse *FsEtcd) Dump(l int, dir *DirInfo, pn path.Tpathname, p sp.Tpath) error {
	s := ""
	for i := 0; i < l*4; i++ {
		s += " "
	}
	dir.Ents.Iter(func(name string, di *DirEntInfo) bool {
		if name != "." {
			fmt.Printf("%v%v %v\n", s, pn.Append(name), di)
			if di.Perm.IsDir() {
				nd, _, _, err := fse.readDir(di, TSTAT_NONE)
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
