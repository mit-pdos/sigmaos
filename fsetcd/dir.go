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
		return fmt.Sprintf("{p %v perm %v cid %v lid %v len %d}", di.Path, di.Perm, di.Nf.TclntId(), di.Nf.TLeaseID(), len(di.Nf.Data))
	} else {
		return fmt.Sprintf("{p %v perm %v}", di.Path, di.Perm)
	}
}

func (di *DirEntInfo) isNotEmpty() bool {
	if di.Nf.Tperm().IsDir() {
		if dir, err := UnmarshalDir(di.Nf.Data); err != nil {
			db.DFatalf("isNonEmptyDir: unmarshalDir %v err %v\n", di.Path, err)
		} else if len(dir.Ents) > 1 {
			return true
		}
	}
	return false
}

type DirInfo struct {
	Ents *sorteddir.SortedDir
	Perm sp.Tperm
}

func (ec *EtcdClnt) MkRootDir() *serr.Err {
	nf, r := MkEtcdFileDir(sp.DMDIR, ROOT, sp.NoClntId)
	if r != nil {
		return serr.MkErrError(r)
	}
	if err := ec.PutFile(ROOT, nf); err != nil {
		return err
	}
	db.DPrintf(db.ETCDCLNT, "mkRoot: PutFile %v\n", nf)
	return nil
}

func (ec *EtcdClnt) ReadRootDir() (*DirInfo, *serr.Err) {
	return ec.ReadDir(ROOT)
}

func (ec *EtcdClnt) Lookup(d sessp.Tpath, name string) (DirEntInfo, *serr.Err) {
	dir, _, err := ec.readDir(d, false)
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
func (ec *EtcdClnt) Create(d sessp.Tpath, name string, path sessp.Tpath, nf *EtcdFile) (DirEntInfo, *serr.Err) {
	dir, v, err := ec.readDir(d, false)
	if err != nil {
		return DirEntInfo{}, err
	}
	_, ok := dir.Ents.Lookup(name)
	if ok {
		return DirEntInfo{}, serr.MkErr(serr.TErrExists, name)
	}
	dir.Ents.Insert(name, DirEntInfo{Nf: nf, Path: path, Perm: nf.Tperm()})
	db.DPrintf(db.ETCDCLNT, "Create %v dir %v nf %v\n", name, dir, nf)
	if err := ec.create(d, dir, v, path, nf); err == nil {
		di := DirEntInfo{Nf: nf, Perm: nf.Tperm(), Path: path}
		return di, nil
	} else {
		return DirEntInfo{}, err
	}
}

func (ec *EtcdClnt) ReadDir(d sessp.Tpath) (*DirInfo, *serr.Err) {
	dir, _, err := ec.readDir(d, true)
	if err != nil {
		return nil, err
	}
	dir.Ents.Delete(".")
	return dir, nil
}

func (ec *EtcdClnt) Remove(d sessp.Tpath, name string) *serr.Err {
	dir, v, err := ec.readDir(d, false)
	if err != nil {
		return err
	}
	e, ok := dir.Ents.Lookup(name)
	if !ok {
		return serr.MkErr(serr.TErrNotfound, name)
	}

	di := e.(DirEntInfo)
	db.DPrintf(db.ETCDCLNT, "Remove in %v entry %v %v v %v\n", dir, name, di, v)

	if di.isNotEmpty() {
		return serr.MkErr(serr.TErrNotEmpty, name)
	}

	dir.Ents.Delete(name)

	if err := ec.remove(d, dir, v, di.Path); err != nil {
		return err
	}
	return nil
}

func (ec *EtcdClnt) Rename(d sessp.Tpath, from, to string) *serr.Err {
	dir, v, err := ec.readDir(d, false)
	if err != nil {
		return err
	}
	db.DPrintf(db.ETCDCLNT, "Rename in %v from %v to %v\n", dir, from, to)
	fromi, ok := dir.Ents.Lookup(from)
	if !ok {
		return serr.MkErr(serr.TErrNotfound, from)
	}
	difrom := fromi.(DirEntInfo)
	topath := sessp.Tpath(0)
	toi, ok := dir.Ents.Lookup(to)
	if ok {
		di := toi.(DirEntInfo)
		if di.isNotEmpty() {
			return serr.MkErr(serr.TErrNotEmpty, to)
		}
		topath = di.Path
	}
	if ok {
		dir.Ents.Delete(to)
	}
	dir.Ents.Delete(from)
	dir.Ents.Insert(to, difrom)
	return ec.rename(d, dir, v, topath)
}

func (ec *EtcdClnt) Renameat(df sessp.Tpath, from string, dt sessp.Tpath, to string) *serr.Err {
	dirf, vf, err := ec.readDir(df, false)
	if err != nil {
		return err
	}
	dirt, vt, err := ec.readDir(dt, false)
	if err != nil {
		return err
	}
	db.DPrintf(db.ETCDCLNT, "Renameat %v dir: %v %v %v %v\n", df, dirf, dirt, vt, vf)
	fi, ok := dirf.Ents.Lookup(from)
	if !ok {
		return serr.MkErr(serr.TErrNotfound, from)
	}
	difrom := fi.(DirEntInfo)
	topath := sessp.Tpath(0)
	ti, ok := dirt.Ents.Lookup(to)
	if ok {
		di := ti.(DirEntInfo)
		if di.isNotEmpty() {
			return serr.MkErr(serr.TErrNotEmpty, to)
		}
		topath = di.Path
	}
	if ok {
		dirt.Ents.Delete(to)
	}
	dirf.Ents.Delete(from)
	dirt.Ents.Insert(to, difrom)
	return ec.renameAt(df, dirf, vf, dt, dirt, vt, topath)
}

func (ec *EtcdClnt) Dump(l int, dir *DirInfo, pn path.Path, p sessp.Tpath) error {
	s := ""
	for i := 0; i < l*4; i++ {
		s += " "
	}
	dir.Ents.Iter(func(name string, v interface{}) bool {
		if name != "." {
			di := v.(DirEntInfo)
			fmt.Printf("%v%v %v\n", s, pn.Append(name), di)
			if di.Perm.IsDir() {
				nd, _, err := ec.readDir(di.Path, false)
				if err == nil {
					ec.Dump(l+1, nd, pn.Append(name), di.Path)
				} else {
					log.Printf("dumpDir: getObj %v %v\n", name, err)
				}
			}
		}
		return true
	})
	return nil
}
