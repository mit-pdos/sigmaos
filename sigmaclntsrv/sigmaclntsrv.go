// Package sigmaclntsrv is an RPC-based server that proxies the
// [sigmaos] interface over a pipe; it reads requests on stdin and
// write responses to stdout.
package sigmaclntsrv

import (
	"io"
	"os"

	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/frame"
	"sigmaos/fs"
	"sigmaos/netsigma"
	"sigmaos/proc"
	"sigmaos/rpcsrv"
	// "sigmaos/serr"
	"sigmaos/sigmaclnt"
	scproto "sigmaos/sigmaclntsrv/proto"
	sp "sigmaos/sigmap"
)

type RPCCh struct {
	req  io.Reader
	rep  io.Writer
	rpcs *rpcsrv.RPCSrv
	ctx  fs.CtxI
}

// SigmaClntSrv exports the RPC methods that the server proxies.  The
// RPC methods correspond to the functions in the sigmaos interface.
type SigmaClntSrv struct {
	sc *sigmaclnt.SigmaClnt
}

func NewSigmaClntSrv() (*SigmaClntSrv, error) {
	localIP, err := netsigma.LocalIP()
	if err != nil {
		db.DFatalf("Error local IP: %v", err)
	}
	pcfg := proc.NewTestProcEnv(sp.ROOTREALM, "127.0.0.1", localIP, "local-build", false)
	sc, err := sigmaclnt.NewSigmaClntRootInit(pcfg)
	if err != nil {
		return nil, err
	}
	scs := &SigmaClntSrv{sc}
	return scs, nil
}

func (scs *SigmaClntSrv) Close(ctx fs.CtxI, req scproto.SigmaCloseRequest, rep *scproto.SigmaErrReply) error {
	err := scs.sc.Close(int(req.Fd))
	db.DPrintf(db.SIGMACLNTSRV, "Close %v err %v\n", req, err)
	if err == nil {
		rep.Err = sp.NewRerror()
	} else {
		rep.Err = sp.NewRerrorErr(err)
	}
	return nil
}

func (scs *SigmaClntSrv) Create(ctx fs.CtxI, req scproto.SigmaCreateRequest, rep *scproto.SigmaFdReply) error {
	fd, err := scs.sc.Create(req.Path, sp.Tperm(req.Perm), sp.Tmode(req.Mode))
	db.DPrintf(db.SIGMACLNTSRV, "Create %v %v err %v\n", req, fd, err)
	if err == nil {
		rep.Err = sp.NewRerror()
	} else {
		rep.Err = sp.NewRerrorErr(err)
	}
	return nil
}

func (scs *SigmaClntSrv) Stat(ctx fs.CtxI, req scproto.SigmaStatRequest, rep *scproto.SigmaStatReply) error {
	st, err := scs.sc.Stat(req.Path)
	db.DPrintf(db.SIGMACLNTSRV, "Stat %v %v %v\n", req, st, err)
	rep.Stat = st
	if err == nil {
		rep.Err = sp.NewRerror()
	} else {
		rep.Err = sp.NewRerrorErr(err)
	}
	return nil
}

func (rpcch *RPCCh) serveRPC() error {
	f, err := frame.ReadFrame(rpcch.req)
	if err != nil {
		db.DPrintf(db.SIGMACLNTSRV, "ReadFrame err %v", err)
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
	scs, err := NewSigmaClntSrv()
	if err != nil {
		return err
	}
	rpcs := rpcsrv.NewRPCSrv(scs, nil)
	rpcch := &RPCCh{os.Stdin, os.Stdout, rpcs, ctx.NewCtxNull()}
	if err := rpcch.serveRPC(); err != nil {
		db.DPrintf(db.SIGMACLNTSRV, "Handle err %v\n", err)
	}
	return nil
}
