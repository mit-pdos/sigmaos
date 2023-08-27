package fsetcd

import (
	"context"
	"time"

	"go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/proto"

	"sigmaos/config"
	db "sigmaos/debug"
	"sigmaos/proc"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

const (
	DialTimeout = 5 * time.Second
	SessionTTL  = 5
	LeaseTTL    = SessionTTL // 30
)

var (
	endpointsBase = []string{":2379", ":22379", ":32379"}
)

type FsEtcd struct {
	*clientv3.Client
	fencekey string
	fencerev int64
	scfg     *config.SigmaConfig
}

func MkFsEtcd(scfg *config.SigmaConfig) (*FsEtcd, error) {
	endpoints := []string{}
	for i := range endpointsBase {
		endpoints = append(endpoints, scfg.EtcdIP+endpointsBase[i])
	}
	// XXX TODO remove
	db.DPrintf(db.ALWAYS, "Etcd addrs %v", endpoints)
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: DialTimeout,
	})
	if err != nil {
		return nil, err
	}
	fs := &FsEtcd{Client: cli, scfg: scfg}
	return fs, nil
}

func (fs *FsEtcd) Close() error {
	return fs.Client.Close()
}

func (fs *FsEtcd) Fence(key string, rev int64) {
	db.DPrintf(db.FSETCD, "%v: Fence key %v rev %d\n", scfg.PID, key, rev)
	fs.fencekey = key
	fs.fencerev = rev
}

func (fs *FsEtcd) Detach(cid sp.TclntId) {
}

func (fs *FsEtcd) SetRootNamed(mnt sp.Tmount) *serr.Err {
	db.DPrintf(db.FSETCD, "SetRootNamed %v", mnt)
	d, err := mnt.Marshal()
	if err != nil {
		return serr.MkErrError(err)
	}
	nf := MkEtcdFile(sp.DMSYMLINK, sp.NoClntId, sp.NoLeaseId, d)
	if b, err := proto.Marshal(nf); err != nil {
		return serr.MkErrError(err)
	} else {
		cmp := []clientv3.Cmp{
			clientv3.Compare(clientv3.CreateRevision(fs.fencekey), "=", fs.fencerev),
		}
		ops := []clientv3.Op{
			clientv3.OpPut(fs.path2key(BOOT), string(b)),
		}
		resp, err := fs.Txn(context.TODO()).If(cmp...).Then(ops...).Commit()
		if err != nil {
			db.DPrintf(db.FSETCD, "SetNamed txn %v err %v\n", nf, err)
			return serr.MkErrError(err)
		}
		db.DPrintf(db.FSETCD, "SetNamed txn %v %v\n", nf, resp)
		return nil
	}
}

func GetRootNamed(scfg *config.SigmaConfig) (sp.Tmount, *serr.Err) {
	fs, err := MkFsEtcd(scfg)
	if err != nil {
		return sp.Tmount{}, serr.MkErrError(err)
	}
	defer fs.Close()
	nf, _, sr := fs.GetFile(sp.Tpath(BOOT))
	if sr != nil {
		db.DPrintf(db.FSETCD, "GetFile %v nf %v err %v conf %v", BOOT, nf, sr, scfg)
		return sp.Tmount{}, sr
	}
	mnt, sr := sp.MkMount(nf.Data)
	if sr != nil {
		db.DPrintf(db.FSETCD, "MkMount %v err %v\n", BOOT, err)
		return sp.Tmount{}, sr
	}
	db.DPrintf(db.FSETCD, "GetNamed mnt %v\n", mnt)
	return mnt, nil
}
