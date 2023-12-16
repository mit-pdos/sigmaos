package sigmaclntsrv

import (
	"io"
	"os"

	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/frame"
	"sigmaos/fs"
	// "sigmaos/serr"
	"sigmaos/rpcsrv"
	scproto "sigmaos/sigmaclntsrv/proto"
	// sp "sigmaos/sigmap"
)

type RPCCh struct {
	req  io.Reader
	rep  io.Writer
	rpcs *rpcsrv.RPCSrv
	ctx  fs.CtxI
}

type SigmaClntSrv struct{}

func (scs *SigmaClntSrv) Stat(ctx fs.CtxI, req scproto.StatRequest, rep *scproto.StatReply) error {
	db.DPrintf(db.ALWAYS, "Stat %v\n", req)
	return nil
}

func (rpcch *RPCCh) serveRPC() error {
	f, err := frame.ReadFrame(rpcch.req)
	if err != nil {
		db.DPrintf(db.ALWAYS, "ReadFrame err %v", err)
		return err
	}
	b, err := rpcch.rpcs.WriteRead(rpcch.ctx, f)
	if err != nil {
		return err
	}
	if err := frame.WriteFrame(rpcch.rep, b); err != nil {
		return err
	}
	return nil
}

func RunSigmaClntSrv(args []string) error {
	rpcs := rpcsrv.NewRPCSrv(&SigmaClntSrv{}, nil)
	rpcch := &RPCCh{os.Stdin, os.Stdout, rpcs, ctx.NewCtxNull()}
	if err := rpcch.serveRPC(); err != nil {
		db.DPrintf(db.ALWAYS, "Handle err %v\n", err)
	}
	return nil
}
