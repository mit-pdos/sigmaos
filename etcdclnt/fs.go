package etcdclnt

import (
	"context"
	"strconv"

	"go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

const (
	BOOT sessp.Tpath = 0
	ROOT sessp.Tpath = 1
)

func path2key(path sessp.Tpath) string {
	return strconv.FormatUint(uint64(path), 16)
}

func key2path(key string) sessp.Tpath {
	p, err := strconv.ParseUint(key, 16, 64)
	if err != nil {
		db.DFatalf("ParseUint %v err %v\n", key, err)
	}
	return sessp.Tpath(p)
}

func MarshalDir(dir *NamedDir) ([]byte, *serr.Err) {
	d, err := proto.Marshal(dir)
	if err != nil {
		return nil, serr.MkErrError(err)
	}
	nfd := &NamedFile{Perm: uint32(sp.DMDIR), Data: d}
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

func GetFile(cli *clientv3.Client, p sessp.Tpath) (*NamedFile, sp.TQversion, *serr.Err) {
	resp, err := cli.Get(context.TODO(), path2key(p))
	if err != nil {
		return nil, 0, serr.MkErrError(err)
	}
	db.DPrintf(db.NAMEDV1, "GetFile %v %v\n", path2key(p), resp)
	if len(resp.Kvs) != 1 {
		return nil, 0, serr.MkErr(serr.TErrNotfound, p)
	}
	nf := &NamedFile{}
	if err := proto.Unmarshal(resp.Kvs[0].Value, nf); err != nil {
		return nil, 0, serr.MkErrError(err)
	}
	db.DPrintf(db.NAMEDV1, "GetFile %v %v\n", path2key(p), nf)
	return nf, sp.TQversion(resp.Kvs[0].Version), nil
}

func PutFile(cli *clientv3.Client, p sessp.Tpath, nf *NamedFile) *serr.Err {
	if b, err := proto.Marshal(nf); err != nil {
		return serr.MkErrError(err)
	} else {
		resp, err := cli.Put(context.TODO(), path2key(p), string(b))
		if err != nil {
			return serr.MkErrError(err)
		}
		db.DPrintf(db.NAMEDV1, "PutFile %v %v %v\n", p, nf, resp)
		return nil
	}
}

func ReadDir(cli *clientv3.Client, p sessp.Tpath) (*NamedDir, sp.TQversion, *serr.Err) {
	db.DPrintf(db.NAMEDV1, "readDir %v\n", p)
	nf, v, err := GetFile(cli, p)
	if err != nil {
		return nil, 0, err
	}
	dir, err := UnmarshalDir(nf.Data)
	if err != nil {
		return nil, 0, err
	}
	return dir, v, nil
}
