package fsetcd

import (
	"strconv"
	"strings"

	"go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

func key2path(key string) sessp.Tpath {
	parts := strings.Split(key, ":")
	p, err := strconv.ParseUint(parts[1], 16, 64)
	if err != nil {
		db.DFatalf("ParseUint %v err %v\n", key, err)
	}
	return sessp.Tpath(p)
}

func marshalDirInfo(dir *DirInfo) ([]byte, *serr.Err) {
	d := &NamedDir{Ents: make([]*DirEnt, dir.Ents.Len())}
	idx := 0
	dir.Ents.Iter(func(name string, i interface{}) bool {
		di := i.(DirEntInfo)
		d.Ents[idx] = &DirEnt{Name: name, Path: uint64(di.Path)}
		idx += 1
		return true
	})
	return marshalDir(d, dir.Perm)
}

func marshalDir(dir *NamedDir, dperm sp.Tperm) ([]byte, *serr.Err) {
	d, err := proto.Marshal(dir)
	if err != nil {
		return nil, serr.MkErrError(err)
	}
	nfd := &NamedFile{Perm: uint32(dperm), Data: d, ClientId: uint64(sp.NoClntId)}
	b, err := proto.Marshal(nfd)
	if err != nil {
		return nil, serr.MkErrError(err)
	}
	return b, nil
}

func UnmarshalDir(b []byte) (*NamedDir, *serr.Err) {
	dir := &NamedDir{}
	if err := proto.Unmarshal(b, dir); err != nil {
		return nil, serr.MkErrError(err)
	}
	return dir, nil
}

func (dir *NamedDir) lookup(name string) (*DirEnt, bool) {
	for _, e := range dir.Ents {
		if e.Name == name {
			return e, true
		}
	}
	return nil, false
}

func MkNamedFile(perm sp.Tperm, cid sp.TclntId, data []byte) *NamedFile {
	return &NamedFile{Perm: uint32(perm), Data: data, ClientId: uint64(cid)}
}

// Make empty file or directory
func MkNamedFileDir(perm sp.Tperm, path sessp.Tpath, cid sp.TclntId) (*NamedFile, error) {
	var fdata []byte
	if perm.IsDir() {
		nd := &NamedDir{}
		nd.Ents = append(nd.Ents, &DirEnt{Name: ".", Path: uint64(path)})
		d, err := proto.Marshal(nd)
		if err != nil {
			return nil, err
		}
		fdata = d
	}
	return MkNamedFile(perm|0777, cid, fdata), nil
}

func (nf *NamedFile) Tperm() sp.Tperm {
	return sp.Tperm(nf.Perm)
}

func (nf *NamedFile) TclntId() sp.TclntId {
	return sp.TclntId(nf.ClientId)
}

func (nf *NamedFile) TLeaseID() clientv3.LeaseID {
	return clientv3.LeaseID(nf.LeaseId)
}

func (nf *NamedFile) SetLeaseId(lid clientv3.LeaseID) {
	nf.LeaseId = int64(lid)
}

func (e *DirEnt) Tpath() sessp.Tpath {
	return sessp.Tpath(e.Path)
}
