package sigmasrv

import (
	"sigmaos/fs"
	"sigmaos/inode"
	"sigmaos/memfssrv"
	"sigmaos/serr"
	"sigmaos/sessp"
)

//
// RPC server, which borrows from go's RPC dispatch
//

type rpcDev struct {
	ssrv *SigmaSrv
}

func mkRpcDev(ssrv *SigmaSrv) *rpcDev {
	return &rpcDev{ssrv}
}

type rpcSession struct {
	*inode.Inode
	ssrv *SigmaSrv
}

func (rd *rpcDev) mkRpcSession(mfs *memfssrv.MemFs, sid sessp.Tsession) (fs.Inode, *serr.Err) {
	rpc := &rpcSession{}
	rpc.ssrv = rd.ssrv
	rpc.Inode = mfs.MakeDevInode()
	return rpc, nil
}

func (rpc *rpcSession) WriteRead(ctx fs.CtxI, b []byte) ([]byte, *serr.Err) {
	return rpc.ssrv.rpcs.WriteRead(ctx, b)
}
