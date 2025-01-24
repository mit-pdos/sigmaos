package fsetcd

import (
	"fmt"
	"strconv"
	"strings"

	"go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type EtcdFile struct {
	*EtcdFileProto
}

func newEtcdFile() *EtcdFile {
	return &EtcdFile{&EtcdFileProto{}}
}

func (ef *EtcdFile) String() string {
	return fmt.Sprintf("{l %d}", len(ef.Data))
}

type EtcdDir struct {
	*EtcdDirProto
}

func newEtcdDir() *EtcdDir {
	return &EtcdDir{&EtcdDirProto{}}
}

type EtcdDirEnt struct {
	*EtcdDirEntProto
}

func prefixLease(realm sp.Trealm) string {
	return LEASEPREFIX + string(realm)
}

func key2path(key string) (sp.Trealm, sp.Tpath) {
	key = strings.TrimPrefix(key, LEASEPREFIX)
	parts := strings.Split(key, ":")
	p, err := strconv.ParseUint(parts[1], 16, 64)
	if err != nil {
		db.DFatalf("ParseUint %v err %v\n", key, err)
	}
	return sp.Trealm(parts[0]), sp.Tpath(p)
}

func key2realmpath(key string) string {
	r, p := key2path(key)
	return r.String() + ":" + p.String()
}

func marshalDirInfo(dir *DirInfo) ([]byte, *serr.Err) {
	d := &EtcdDirProto{Ents: make([]*EtcdDirEntProto, dir.Ents.Len())}
	idx := 0
	dir.Ents.Iter(func(name string, di *DirEntInfo) bool {
		d.Ents[idx] = &EtcdDirEntProto{
			Name:     name,
			Path:     uint64(di.Path),
			Perm:     uint32(di.Perm),
			ClientId: uint64(di.ClntId),
			LeaseId:  int64(di.LeaseId),
		}
		idx += 1
		return true
	})
	return marshalDir(d)
}

func marshalDir(dir *EtcdDirProto) ([]byte, *serr.Err) {
	d, err := proto.Marshal(dir)
	if err != nil {
		return nil, serr.NewErrError(err)
	}
	nfd := &EtcdFileProto{Data: d}
	b, err := proto.Marshal(nfd)
	if err != nil {
		return nil, serr.NewErrError(err)
	}
	return b, nil
}

func UnmarshalDir(b []byte) (*EtcdDir, *serr.Err) {
	dir := newEtcdDir()
	if err := proto.Unmarshal(b, dir.EtcdDirProto); err != nil {
		return nil, serr.NewErrError(err)
	}
	return dir, nil
}

func (dir *EtcdDir) lookup(name string) (*EtcdDirEnt, bool) {
	for _, e := range dir.Ents {
		if e.Name == name {
			return &EtcdDirEnt{e}, true
		}
	}
	return nil, false
}

func NewEtcdFile(data []byte) *EtcdFile {
	return &EtcdFile{
		&EtcdFileProto{Data: data},
	}
}

// Make empty file or directory
func NewEtcdFileDir(perm sp.Tperm, path sp.Tpath, cid sp.TclntId, lid sp.TleaseId) (*EtcdFile, error) {
	var fdata []byte
	perm = perm | 0777
	if perm.IsDir() {
		nd := &EtcdDir{&EtcdDirProto{}}
		nd.Ents = append(nd.Ents, &EtcdDirEntProto{
			Name:     ".",
			Path:     uint64(path),
			Perm:     uint32(perm),
			ClientId: uint64(cid),
			LeaseId:  int64(lid),
		})
		d, err := proto.Marshal(nd.EtcdDirProto)
		if err != nil {
			return nil, err
		}
		fdata = d
	}
	return NewEtcdFile(fdata), nil
}

func (e *DirEntInfo) LeaseOpts() []clientv3.OpOption {
	opts := make([]clientv3.OpOption, 0)
	if e.LeaseId.IsLeased() {
		opts = append(opts, clientv3.WithLease(clientv3.LeaseID(e.LeaseId)))
	}
	return opts

}

func (e *EtcdDirEntProto) Tpath() sp.Tpath {
	return sp.Tpath(e.Path)
}

func (e *EtcdDirEntProto) Tperm() sp.Tperm {
	return sp.Tperm(e.Perm)
}

func (nf *EtcdDirEntProto) TclntId() sp.TclntId {
	return sp.TclntId(nf.ClientId)
}

func (nf *EtcdDirEntProto) TleaseId() sp.TleaseId {
	return sp.TleaseId(nf.LeaseId)
}
