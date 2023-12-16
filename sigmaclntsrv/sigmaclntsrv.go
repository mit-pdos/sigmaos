package sigmaclntsrv

import (
	"io"
	"os"

	"google.golang.org/protobuf/proto"

	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/frame"
	"sigmaos/fs"
	rpcproto "sigmaos/rpc/proto"
	// "sigmaos/serr"
	"sigmaos/rpcsrv"
	scproto "sigmaos/sigmaclntsrv/proto"
	sp "sigmaos/sigmap"
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
	req := rpcproto.Request{}
	if err := proto.Unmarshal(f, &req); err != nil {
		return err
	}

	var rerr *sp.Rerror
	b, sr := rpcch.rpcs.ServeRPC(rpcch.ctx, req.Method, req.Args)
	if sr != nil {
		rerr = sp.NewRerrorSerr(sr)
	} else {
		rerr = sp.NewRerror()
	}

	rep := &rpcproto.Reply{Res: b, Err: rerr}
	b, r := proto.Marshal(rep)
	if r != nil {
		return r
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
