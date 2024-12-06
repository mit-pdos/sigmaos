// The fsetcd package implements the [fs] API on top of etcd.

package fsetcd

import (
	"context"
	"fmt"
	"net"
	"time"

	"go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/dialproxy"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/stats"
)

const (
	DialTimeout = 5 * time.Second
	LeaseTTL    = sp.EtcdSessionTTL
)

var (
	//	endpointPorts = []sp.Tport{3379, 3380, 3381, 3382, 3383} // For use when running on the same machine as K8s
	endpointPorts = []sp.Tport{2379, 2380, 2381, 2382, 2383}
)

type TetcdEndpoints map[string]*sp.TendpointProto

type FsEtcd struct {
	*clientv3.Client
	fencekey string
	fencerev int64
	realm    sp.Trealm
	dc       *Dcache
	pstats   *pstats
}

func NewFsEtcdEndpoint(ip sp.Tip) (TetcdEndpoints, error) {
	eps := map[string]*sp.TendpointProto{}
	for i := range endpointPorts {
		addr := sp.NewTaddr(ip, sp.INNER_CONTAINER_IP, endpointPorts[i])
		ep := sp.NewEndpoint(sp.EXTERNAL_EP, []*sp.Taddr{addr})
		eps[addr.IPPort()] = ep.GetProto()
	}
	return eps, nil
}

func NewFsEtcd(dial dialproxy.DialFn, etcdMnts map[string]*sp.TendpointProto, realm sp.Trealm, psi *PstatInode) (*FsEtcd, error) {
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
	if psi != nil {
		fs.pstats = psi.pstats
	}
	return fs, nil
}

func (fs *FsEtcd) WatchLeased(ch chan path.Tpathname) error {
	wopts := make([]clientv3.OpOption, 0)
	wopts = append(wopts, clientv3.WithPrefix())
	wopts = append(wopts, clientv3.WithFilterPut())
	wopts = append(wopts, clientv3.WithPrevKV())
	wch := fs.Client.Watch(context.TODO(), prefixLease(fs.realm), wopts...)
	if wch == nil {
		return fmt.Errorf("watchLeased: Watch failed")
	}
	db.DPrintf(db.WATCH, "WatchLeased: %v Set up etcd watch for %v", fs.realm, prefixLease(fs.realm))

	go func() error {
		for {
			watchResp, ok := <-wch
			if ok {
				for _, e := range watchResp.Events {
					key := string(e.Kv.Key)
					pn := string(e.PrevKv.Value)
					db.DPrintf(db.FSETCD, "WatchLeased: %v watchResp event %v", fs.realm, key)
					db.DPrintf(db.WATCH, "WatchLeased: %v expired leased file '%v'", fs.realm, pn)
					ch <- path.Split(pn)
				}
			} else {
				db.DPrintf(db.FSETCD, "WatchLeased: wch closed\n")
				return nil
			}
		}
	}()
	return nil
}

func (fs *FsEtcd) StopWatch() error {
	return fs.Client.Watcher.Close()
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

func (fs *FsEtcd) PstatUpdate(pn path.Tpathname, c stats.Tcounter) {
	if fs.pstats == nil {
		return
	}
	fs.pstats.Update(pn, c)
}

func (fs *FsEtcd) SetRootNamed(ep *sp.Tendpoint) *serr.Err {
	db.DPrintf(db.FSETCD, "SetRootNamed %v", ep)
	d, err := ep.Marshal()
	if err != nil {
		return serr.NewErrError(err)
	}
	nf := NewEtcdFile(sp.DMSYMLINK, d)
	if b, err := proto.Marshal(nf.EtcdFileProto); err != nil {
		return serr.NewErrError(err)
	} else {
		dei := NewDirEntInfoDir(sp.Tpath(BOOT))
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
		// XXX return failure if fence check fails
		db.DPrintf(db.FSETCD, "SetNamed txn %v %v\n", nf, resp)
		return nil
	}
}

func GetRootNamed(dial dialproxy.DialFn, etcdMnts map[string]*sp.TendpointProto, realm sp.Trealm) (*sp.Tendpoint, *serr.Err) {
	fs, err := NewFsEtcd(dial, etcdMnts, realm, nil)
	if err != nil {
		return &sp.Tendpoint{}, serr.NewErrError(err)
	}
	defer fs.Close()
	dei := NewDirEntInfoDir(sp.Tpath(BOOT))
	nf, _, _, sr := fs.getFile(fs.path2key(sp.ROOTREALM, dei))
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
