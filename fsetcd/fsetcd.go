package fsetcd

import (
	"context"
	"time"

	"go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/proc"
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

type FsEtcd struct {
	*clientv3.Client
	realm    sp.Trealm
	fencekey string
	fencerev int64
	lmgr     *leaseMgr
}

func MkFsEtcd(r sp.Trealm) (*FsEtcd, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: DialTimeout,
	})
	if err != nil {
		return nil, err
	}
	fs := &FsEtcd{realm: r, Client: cli}
	fs.lmgr = mkLeaseMgr(fs)
	return fs, nil
}

func (fs *FsEtcd) Close() error {
	fs.lmgr.lc.Close()
	return fs.Client.Close()
}

func (fs *FsEtcd) Recover(cid sp.TclntId) error {
	return fs.lmgr.recoverLeases(cid)
}

func (fs *FsEtcd) Fence(key string, rev int64) {
	db.DPrintf(db.FSETCD, "%v: Fence key %v rev %d\n", proc.GetPid(), key, rev)
	fs.fencekey = key
	fs.fencerev = rev
}

func (fs *FsEtcd) Detach(cid sp.TclntId) {
	fs.lmgr.detach(cid)
}

func (fs *FsEtcd) SetRootNamed(mnt sp.Tmount) *serr.Err {
	d, err := mnt.Marshal()
	if err != nil {
		return serr.MkErrError(err)
	}
	nf := MkEtcdFile(sp.DMSYMLINK, sp.NoClntId, d)
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

func GetRootNamed() (sp.Tmount, *serr.Err) {
	fs, err := MkFsEtcd(sp.ROOTREALM)
	if err != nil {
		return sp.Tmount{}, serr.MkErrError(err)
	}
	defer fs.Close()
	nf, _, sr := fs.GetFile(sessp.Tpath(BOOT))
	if sr != nil {
		db.DPrintf(db.FSETCD, "GetFile %v %v err %v\n", BOOT, nf, sr)
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
