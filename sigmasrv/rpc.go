package sigmasrv

import (
	"sigmaos/fs"
	"sigmaos/inode"
	"sigmaos/memfssrv"
	"sigmaos/rpcsrv"
	"sigmaos/serr"
	"sigmaos/sessp"
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

func (rd *rpcDev) newRpcSession(mfs *memfssrv.MemFs, sid sessp.Tsession) (fs.Inode, *serr.Err) {
	rpc := &rpcSession{rpcs: rd.rpcs, Inode: mfs.NewDevInode()}
	return rpc, nil
}

func (rpc *rpcSession) WriteRead(ctx fs.CtxI, b []byte) ([]byte, *serr.Err) {
	return rpc.rpcs.WriteRead(ctx, b)
}
