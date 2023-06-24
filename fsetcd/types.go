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

func MarshalDir(dir *NamedDir, dperm sp.Tperm) ([]byte, *serr.Err) {
	d, err := proto.Marshal(dir)
	if err != nil {
		return nil, serr.MkErrError(err)
	}
	nfd := &NamedFile{Perm: uint32(dperm), Data: d, SessionId: uint64(sessp.NoSession)}
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

func MkNamedFile(perm sp.Tperm, sid sessp.Tsession, data []byte) *NamedFile {
	return &NamedFile{Perm: uint32(perm), Data: data, SessionId: uint64(sid)}
}

func (nf *NamedFile) Tsession() sessp.Tsession {
	return sessp.Tsession(nf.SessionId)
}

func (nf *NamedFile) TLeaseID() clientv3.LeaseID {
	return clientv3.LeaseID(nf.LeaseId)
}

func (nf *NamedFile) SetLeaseId(lid clientv3.LeaseID) {
	nf.LeaseId = int64(lid)
}
