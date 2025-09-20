package sigmasrv

import (
	"sigmaos/api/fs"
	rpcsrv "sigmaos/rpc/srv"
	"sigmaos/serr"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv/memfssrv"
	"sigmaos/sigmasrv/memfssrv/memfs/inode"
)

//
// RPC server, which borrows from go's RPC dispatch
//

type rpcDev struct {
	rpcs *rpcsrv.RPCSrv
}

func newRpcDev(rpcs *rpcsrv.RPCSrv) *rpcDev {
	return &rpcDev{rpcs}
}

type rpcSession struct {
	*inode.Inode
	rpcs *rpcsrv.RPCSrv
}

func (rd *rpcDev) newRpcSession(mfs *memfssrv.MemFs, sid sessp.Tsession) (fs.FsObj, *serr.Err) {
	rpc := &rpcSession{rpcs: rd.rpcs, Inode: mfs.NewDevInode()}
	return rpc, nil
}

func (rpc *rpcSession) Stat(ctx fs.CtxI) (*sp.Tstat, *serr.Err) {
	st, err := rpc.Inode.NewStat()
	if err != nil {
		return nil, err
	}
	return st, nil
}

func (rpc *rpcSession) WriteRead(ctx fs.CtxI, iov *sessp.IoVec) (*sessp.IoVec, *serr.Err) {
	return rpc.rpcs.WriteRead(ctx, iov)
}
