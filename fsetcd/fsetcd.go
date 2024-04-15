package fsetcd

import (
	"context"
	"net"
	"time"

	"go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"sigmaos/auth"
	db "sigmaos/debug"
	"sigmaos/netsigma"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

const (
	DialTimeout = 5 * time.Second
	LeaseTTL    = sp.EtcdSessionTTL // 30
)

var (
	endpointPorts = []sp.Tport{3379, 3380, 3381, 3382, 3383}
)

type FsEtcd struct {
	*clientv3.Client
	fencekey string
	fencerev int64
	realm    sp.Trealm
	dc       *Dcache
}

func NewFsEtcdMount(as auth.AuthSrv, ip sp.Tip) (map[string]*sp.TmountProto, error) {
	mnts := map[string]*sp.TmountProto{}
	for i := range endpointPorts {
		addr := sp.NewTaddr(ip, sp.INNER_CONTAINER_IP, endpointPorts[i])
		mnt := sp.NewMount([]*sp.Taddr{addr}, sp.ROOTREALM)
		if err := as.MintAndSetMountToken(mnt); err != nil {
			db.DPrintf(db.ERROR, "Unable to mint etcd mount token: %v", err)
			return nil, err
		}
		mnts[addr.IPPort()] = mnt.GetProto()
	}
	return mnts, nil
}

func NewFsEtcd(npc *netsigma.NetProxyClnt, etcdMnts map[string]*sp.TmountProto, realm sp.Trealm) (*FsEtcd, error) {
	endpoints := []string{}
	for addr, _ := range etcdMnts {
		endpoints = append(endpoints, addr)
	}
	db.DPrintf(db.FSETCD, "FsEtcd etcd endpoints: %v", endpoints)
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: DialTimeout,
		DialOptions: []grpc.DialOption{grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			mnt, ok := etcdMnts[addr]
			// Check that the mount is in the map
			if !ok {
				db.DFatalf("Unknown fsetcd mount proto: addr %v mnts %v", addr, etcdMnts)
			}
			return npc.Dial(sp.NewMountFromProto(mnt))
		})},
	})
	if err != nil {
		return nil, err
	}
	dc, err := newDcache()
	fs := &FsEtcd{
		Client: cli,
		realm:  realm,
		dc:     dc,
	}
	return fs, nil
}

func (fs *FsEtcd) Close() error {
	return fs.Client.Close()
}

func (fs *FsEtcd) Clnt() *clientv3.Client {
	return fs.Client
}

func (fs *FsEtcd) Fence(key string, rev int64) {
	db.DPrintf(db.FSETCD, "Fence key %v rev %d\n", key, rev)
	fs.fencekey = key
	fs.fencerev = rev
}

func (fs *FsEtcd) Detach(cid sp.TclntId) {
}

func (fs *FsEtcd) SetRootNamed(mnt *sp.Tmount) *serr.Err {
	db.DPrintf(db.FSETCD, "SetRootNamed %v", mnt)
	d, err := mnt.Marshal()
	if err != nil {
		return serr.NewErrError(err)
	}
	nf := NewEtcdFile(sp.DMSYMLINK, sp.NoClntId, sp.NoLeaseId, d)
	if b, err := proto.Marshal(nf); err != nil {
		return serr.NewErrError(err)
	} else {
		cmp := []clientv3.Cmp{
			clientv3.Compare(clientv3.CreateRevision(fs.fencekey), "=", fs.fencerev),
		}
		ops := []clientv3.Op{
			clientv3.OpPut(fs.path2key(sp.ROOTREALM, BOOT), string(b)),
		}
		resp, err := fs.Clnt().Txn(context.TODO()).If(cmp...).Then(ops...).Commit()
		if err != nil {
			db.DPrintf(db.FSETCD, "SetNamed txn %v err %v\n", nf, err)
			return serr.NewErrError(err)
		}
		db.DPrintf(db.FSETCD, "SetNamed txn %v %v\n", nf, resp)
		return nil
	}
}

func GetRootNamed(npc *netsigma.NetProxyClnt, etcdMnts map[string]*sp.TmountProto, realm sp.Trealm) (*sp.Tmount, *serr.Err) {
	fs, err := NewFsEtcd(npc, etcdMnts, realm)
	if err != nil {
		return &sp.Tmount{}, serr.NewErrError(err)
	}
	defer fs.Close()

	nf, _, sr := fs.getFile(fs.path2key(sp.ROOTREALM, sp.Tpath(BOOT)))
	if sr != nil {
		db.DPrintf(db.FSETCD, "GetFile %v nf %v err %v etcdMnt %v realm %v", BOOT, nf, sr, etcdMnts, realm)
		return &sp.Tmount{}, sr
	}
	mnt, sr := sp.NewMountFromBytes(nf.Data)
	if sr != nil {
		db.DPrintf(db.FSETCD, "NewMount %v err %v\n", BOOT, err)
		return &sp.Tmount{}, sr
	}
	db.DPrintf(db.FSETCD, "GetNamed mnt %v\n", mnt)
	fs.Close()
	return mnt, nil
}
