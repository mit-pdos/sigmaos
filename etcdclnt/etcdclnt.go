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
	endpoints = []string{"127.0.0.1:2379", "localhost:22379", "localhost:32379"}
)

type EtcdClnt struct {
	*clientv3.Client
	realm sp.Trealm
}

func MkEtcdClnt(r sp.Trealm) (*EtcdClnt, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: DialTimeout,
	})
	if err != nil {
		return nil, err
	}
	return &EtcdClnt{realm: r, Client: cli}, nil
}

func (ec *EtcdClnt) SetRootNamed(mnt sp.Tmount, key string, rev int64) *serr.Err {
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
		resp, err := ec.Txn(context.TODO()).If(cmp...).Then(ops...).Commit()
		if err != nil {
			return serr.MkErrError(err)
		}
		db.DPrintf(db.ETCDCLNT, "SetNamed txn %v %v\n", nf, resp)
		return nil
	}
}

func GetRootNamed() (sp.Tmount, *serr.Err) {
	ec, err := MkEtcdClnt(sp.ROOTREALM)
	if err != nil {
		return sp.Tmount{}, serr.MkErrError(err)
	}
	defer ec.Close()
	nf, _, sr := ec.GetFile(sessp.Tpath(BOOT))
	if sr != nil {
		db.DPrintf(db.ETCDCLNT, "GetFile %v %v err %v\n", BOOT, nf, sr)
		return sp.Tmount{}, sr
	}
	mnt, sr := sp.MkMount(nf.Data)
	if sr != nil {
		db.DPrintf(db.ETCDCLNT, "MkMount %v err %v\n", BOOT, err)
		return sp.Tmount{}, sr
	}
	db.DPrintf(db.ETCDCLNT, "GetNamed mnt %v\n", mnt)
	return mnt, nil
}
