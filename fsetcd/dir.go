package fsetcd

import (
	"fmt"
	"log"

	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	"sigmaos/sorteddir"
)

const (
	ROOT sessp.Tpath = 1
)

type DirEntInfo struct {
	Nf   *EtcdFile
	Path sessp.Tpath
	Perm sp.Tperm
}

func (di DirEntInfo) String() string {
	if di.Nf != nil {
		return fmt.Sprintf("{p %v perm %v cid %v lid %v len %d}", di.Path, di.Perm, di.Nf.TclntId(), di.Nf.TleaseId(), len(di.Nf.Data))
	} else {
		return fmt.Sprintf("{p %v perm %v}", di.Path, di.Perm)
	}
}

func (di *DirEntInfo) isEmpty() bool {
	if di.Perm.IsDir() {
		if dir, err := UnmarshalDir(di.Nf.Data); err != nil {
			db.DFatalf("isEmptyDir: unmarshalDir %v err %v\n", di.Path, err)
		} else if len(dir.Ents) <= 1 { // don't count "."
			return true
		} else {
			return false
		}
	}
	return true
}

type DirInfo struct {
	Ents *sorteddir.SortedDir
	Perm sp.Tperm
}

func (fs *FsEtcd) isEmpty(di DirEntInfo) (bool, *serr.Err) {
	if di.Nf == nil {
		nf, _, err := fs.GetFile(di.Path)
		if err != nil {
			return false, err
		}
		di.Nf = nf
	}
	return di.isEmpty(), nil
}

func (fs *FsEtcd) MkRootDir() *serr.Err {
	nf, r := MkEtcdFileDir(sp.DMDIR, ROOT, sp.NoClntId, sp.NoLeaseId)
	if r != nil {
		return serr.MkErrError(r)
	}
	if err := fs.PutFile(ROOT, nf); err != nil {
		return err
	}
	db.DPrintf(db.FSETCD, "mkRoot: PutFile %v\n", nf)
	return nil
}

func (fs *FsEtcd) ReadRootDir() (*DirInfo, *serr.Err) {
	return fs.ReadDir(ROOT)
}

func (fs *FsEtcd) Lookup(d sessp.Tpath, name string) (DirEntInfo, *serr.Err) {
	dir, _, err := fs.readDir(d, false)
	if err != nil {
		return DirEntInfo{}, err
	}
	e, ok := dir.Ents.Lookup(name)
	if ok {
		return e.(DirEntInfo), nil
	}
	return DirEntInfo{}, serr.MkErr(serr.TErrNotfound, name)
}

// XXX retry on version mismatch
// OEXCL: should only succeed if file doesn't exist
func (fs *FsEtcd) Create(d sessp.Tpath, name string, path sessp.Tpath, nf *EtcdFile) (DirEntInfo, *serr.Err) {
	dir, v, err := fs.readDir(d, false)
	if err != nil {
		return DirEntInfo{}, err
	}
	_, ok := dir.Ents.Lookup(name)
	if ok {
		return DirEntInfo{}, serr.MkErr(serr.TErrExists, name)
	}
	dir.Ents.Insert(name, DirEntInfo{Nf: nf, Path: path, Perm: nf.Tperm()})
	db.DPrintf(db.FSETCD, "Create %q dir %v nf %v\n", name, dir, nf)
	if err := fs.create(d, dir, v, path, nf); err == nil {
		di := DirEntInfo{Nf: nf, Perm: nf.Tperm(), Path: path}
		return di, nil
	} else {
		return DirEntInfo{}, err
	}
}

func (fs *FsEtcd) ReadDir(d sessp.Tpath) (*DirInfo, *serr.Err) {
	dir, _, err := fs.readDir(d, true)
	if err != nil {
		return nil, err
	}
	dir.Ents.Delete(".")
	return dir, nil
}

func (fs *FsEtcd) Remove(d sessp.Tpath, name string) *serr.Err {
	dir, v, err := fs.readDir(d, false)
	if err != nil {
		return err
	}
	e, ok := dir.Ents.Lookup(name)
	if !ok {
		return serr.MkErr(serr.TErrNotfound, name)
	}

	di := e.(DirEntInfo)
	db.DPrintf(db.FSETCD, "Remove in %v entry %v %v v %v\n", dir, name, di, v)

	empty, err := fs.isEmpty(di)
	if err != nil {
		return err
	}
	if !empty {
		return serr.MkErr(serr.TErrNotEmpty, name)
	}

	dir.Ents.Delete(name)

	if err := fs.remove(d, dir, v, di.Path); err != nil {
		return err
	}
	return nil
}

func (fs *FsEtcd) Rename(d sessp.Tpath, from, to string) *serr.Err {
	dir, v, err := fs.readDir(d, false)
	if err != nil {
		return err
	}
	db.DPrintf(db.FSETCD, "Rename in %v from %v to %v\n", dir, from, to)
	fromi, ok := dir.Ents.Lookup(from)
	if !ok {
		return serr.MkErr(serr.TErrNotfound, from)
	}
	difrom := fromi.(DirEntInfo)
	topath := sessp.Tpath(0)
	toi, ok := dir.Ents.Lookup(to)
	if ok {
		di := toi.(DirEntInfo)
		empty, err := fs.isEmpty(di)
		if err != nil {
			return err
		}
		if !empty {
			return serr.MkErr(serr.TErrNotEmpty, to)
		}
		topath = di.Path
	}
	if ok {
		dir.Ents.Delete(to)
	}
	dir.Ents.Delete(from)
	dir.Ents.Insert(to, difrom)
	return fs.rename(d, dir, v, topath)
}

func (fs *FsEtcd) Renameat(df sessp.Tpath, from string, dt sessp.Tpath, to string) *serr.Err {
	dirf, vf, err := fs.readDir(df, false)
	if err != nil {
		return err
	}
	dirt, vt, err := fs.readDir(dt, false)
	if err != nil {
		return err
	}
	db.DPrintf(db.FSETCD, "Renameat %v dir: %v %v %v %v\n", df, dirf, dirt, vt, vf)
	fi, ok := dirf.Ents.Lookup(from)
	if !ok {
		return serr.MkErr(serr.TErrNotfound, from)
	}
	difrom := fi.(DirEntInfo)
	topath := sessp.Tpath(0)
	ti, ok := dirt.Ents.Lookup(to)
	if ok {
		di := ti.(DirEntInfo)
		empty, err := fs.isEmpty(di)
		if err != nil {
			return err
		}
		if !empty {
			return serr.MkErr(serr.TErrNotEmpty, to)
		}
		topath = di.Path
	}
	if ok {
		dirt.Ents.Delete(to)
	}
	dirf.Ents.Delete(from)
	dirt.Ents.Insert(to, difrom)
	return fs.renameAt(df, dirf, vf, dt, dirt, vt, topath)
}

func (fs *FsEtcd) Dump(l int, dir *DirInfo, pn path.Path, p sessp.Tpath) error {
	s := ""
	for i := 0; i < l*4; i++ {
		s += " "
	}
	dir.Ents.Iter(func(name string, v interface{}) bool {
		if name != "." {
			di := v.(DirEntInfo)
			fmt.Printf("%v%v %v\n", s, pn.Append(name), di)
			if di.Perm.IsDir() {
				nd, _, err := fs.readDir(di.Path, false)
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
