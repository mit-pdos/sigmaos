package fsetcd

import (
	db "sigmaos/debug"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

func (ec *EtcdClnt) Lookup(d sessp.Tpath, name string) (sessp.Tpath, *NamedFile, *serr.Err) {
	dir, _, err := ec.ReadDir(d)
	if err != nil {
		return sessp.NoPath, nil, err
	}
	e, ok := dir.lookup(name)
	if ok {
		nf, _, err := ec.GetFile(sessp.Tpath(e.Path))
		if err != nil {
			return sessp.NoPath, nil, err
		}
		return sessp.Tpath(e.Path), nf, nil
	}
	return sessp.NoPath, nil, serr.MkErr(serr.TErrNotfound, name)
}

func (ec *EtcdClnt) Create(d sessp.Tpath, dperm sp.Tperm, name string, path sessp.Tpath, nf *NamedFile) *serr.Err {
	dir, v, err := ec.ReadDir(d)
	if err != nil {
		return err
	}
	_, ok := dir.lookup(name)
	if ok {
		return serr.MkErr(serr.TErrExists, name)
	}
	dir.Ents = append(dir.Ents, &DirEnt{Name: name, Path: uint64(path)})
	db.DPrintf(db.ETCDCLNT, "Create %v dir %v nf %v\n", name, dir, nf)
	return ec.create(d, dir, dperm, v, path, nf)
}
