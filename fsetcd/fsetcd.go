// The fsetcd package implements the [fs] API on top of etcd.

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
	"sigmaos/netproxy"
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

type TetcdEndpoints map[string]*sp.TendpointProto

type FsEtcd struct {
	*clientv3.Client
	fencekey string
	fencerev int64
	realm    sp.Trealm
	dc       *Dcache
}

func NewFsEtcdEndpoint(amgr auth.AuthMgr, ip sp.Tip) (TetcdEndpoints, error) {
	eps := map[string]*sp.TendpointProto{}
	for i := range endpointPorts {
		addr := sp.NewTaddr(ip, sp.INNER_CONTAINER_IP, endpointPorts[i])
		ep := sp.NewEndpoint([]*sp.Taddr{addr}, sp.ROOTREALM)
		if err := amgr.MintAndSetEndpointToken(ep); err != nil {
			db.DPrintf(db.ERROR, "Unable to mint etcd endpoint token: %v", err)
			return nil, err
		}
		eps[addr.IPPort()] = ep.GetProto()
	}
	return eps, nil
}

func NewFsEtcd(dial netproxy.DialFn, etcdMnts map[string]*sp.TendpointProto, realm sp.Trealm) (*FsEtcd, error) {
	endpoints := []string{}
	for addr, _ := range etcdMnts {
		endpoints = append(endpoints, addr)
	}
	db.DPrintf(db.FSETCD, "FsEtcd etcd endpoints: %v", endpoints)
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: DialTimeout,
		DialOptions: []grpc.DialOption{grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			ep, ok := etcdMnts[addr]
			// Check that the endpoint is in the map
			if !ok {
				db.DFatalf("Unknown fsetcd endpoint proto: addr %v eps %v", addr, etcdMnts)
			}
			return dial(sp.NewEndpointFromProto(ep))
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

func (fs *FsEtcd) SetRootNamed(ep *sp.Tendpoint) *serr.Err {
	db.DPrintf(db.FSETCD, "SetRootNamed %v", ep)
	d, err := ep.Marshal()
	if err != nil {
		return serr.NewErrError(err)
	}
	nf := NewEtcdFile(sp.DMSYMLINK, sp.NoClntId, sp.NoLeaseId, d)
	if b, err := proto.Marshal(nf); err != nil {
		return serr.NewErrError(err)
	} else {
		dei := newDirEntInfoP(sp.Tpath(BOOT), sp.DMDIR)
		cmp := []clientv3.Cmp{
			clientv3.Compare(clientv3.CreateRevision(fs.fencekey), "=", fs.fencerev),
		}
		ops := []clientv3.Op{
			clientv3.OpPut(fs.path2key(sp.ROOTREALM, dei), string(b)),
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

func GetRootNamed(dial netproxy.DialFn, etcdMnts map[string]*sp.TendpointProto, realm sp.Trealm) (*sp.Tendpoint, *serr.Err) {
	fs, err := NewFsEtcd(dial, etcdMnts, realm)
	if err != nil {
		return &sp.Tendpoint{}, serr.NewErrError(err)
	}
	defer fs.Close()
	dei := newDirEntInfoP(sp.Tpath(BOOT), sp.DMDIR)
	nf, _, sr := fs.getFile(fs.path2key(sp.ROOTREALM, dei))
	if sr != nil {
		db.DPrintf(db.FSETCD, "GetFile %v nf %v err %v etcdMnt %v realm %v", BOOT, nf, sr, etcdMnts, realm)
		return &sp.Tendpoint{}, sr
	}
	ep, err := sp.NewEndpointFromBytes(nf.Data)
	if sr != nil {
		db.DPrintf(db.FSETCD, "NewEndpoint %v err %v\n", BOOT, err)
		return &sp.Tendpoint{}, serr.NewErrError(err)
	}
	db.DPrintf(db.FSETCD, "GetNamed ep %v\n", ep)
	fs.Close()
	return ep, nil
}
