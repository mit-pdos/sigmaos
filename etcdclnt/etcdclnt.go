package etcdclnt

import (
	"context"
	"time"

	"go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

const (
	DialTimeout = 5 * time.Second
	SessionTTL  = 5
)

var (
	Endpoints = []string{"127.0.0.1:2379", "localhost:22379", "localhost:32379"}
)

func SetNamed(cli *clientv3.Client, mnt sp.Tmount, key string, rev int64) *serr.Err {
	d, err := mnt.Marshal()
	if err != nil {
		return serr.MkErrError(err)
	}
	nf := &NamedFile{Perm: uint32(sp.DMSYMLINK), Data: d}
	if b, err := proto.Marshal(nf); err != nil {
		return serr.MkErrError(err)
	} else {
		cmp := []clientv3.Cmp{
			clientv3.Compare(clientv3.CreateRevision(key), "=", rev),
		}
		ops := []clientv3.Op{
			clientv3.OpPut(path2key(BOOT), string(b)),
		}
		resp, err := cli.Txn(context.TODO()).If(cmp...).Then(ops...).Commit()
		if err != nil {
			return serr.MkErrError(err)
		}
		db.DPrintf(db.NAMEDV1, "SetNamed txn %v %v\n", nf, resp)
		return nil
	}
}

func GetNamed() (sp.Tmount, *serr.Err) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   Endpoints,
		DialTimeout: DialTimeout,
	})
	if err != nil {
		return sp.Tmount{}, serr.MkErrError(err)
	}
	defer cli.Close()
	nf, _, sr := GetFile(cli, sessp.Tpath(BOOT))
	if sr != nil {
		db.DPrintf(db.NAMEDV1, "GetFile %v %v err %v\n", BOOT, nf, sr)
		return sp.Tmount{}, sr
	}
	mnt, sr := sp.MkMount(nf.Data)
	if sr != nil {
		db.DPrintf(db.NAMEDV1, "MkMount %v err %v\n", BOOT, err)
		return sp.Tmount{}, sr
	}
	db.DPrintf(db.NAMEDV1, "GetNamed mnt %v\n", mnt)
	return mnt, nil
}
