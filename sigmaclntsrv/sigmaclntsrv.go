// Package sigmaclntsrv is an RPC-based server that proxies the
// [sigmaos] interface over a pipe; it reads requests on stdin and
// write responses to stdout.
package sigmaclntsrv

import (
	"errors"
	"os"

	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/fs"
	"sigmaos/netsigma"
	"sigmaos/proc"
	"sigmaos/rpcsrv"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	scproto "sigmaos/sigmaclntsrv/proto"
	sos "sigmaos/sigmaos"
	sp "sigmaos/sigmap"
)

type RPCCh struct {
	dmx  *demux.DemuxSrv
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

func (scs *SigmaClntSrv) setErr(err error) *sp.Rerror {
	if err == nil {
		return sp.NewRerror()
	} else {
		var sr *serr.Err
		if errors.As(err, &sr) {
			return sp.NewRerrorSerr(sr)
		} else {
			return sp.NewRerrorErr(err)
		}
	}
}

func (scs *SigmaClntSrv) Close(ctx fs.CtxI, req scproto.SigmaCloseRequest, rep *scproto.SigmaErrReply) error {
	err := scs.sc.Close(int(req.Fd))
	db.DPrintf(db.SIGMACLNTSRV, "Close %v err %v\n", req, err)
	rep.Err = scs.setErr(err)
	return nil
}

func (scs *SigmaClntSrv) Stat(ctx fs.CtxI, req scproto.SigmaPathRequest, rep *scproto.SigmaStatReply) error {
	st, err := scs.sc.Stat(req.Path)
	db.DPrintf(db.SIGMACLNTSRV, "Stat %v st %v err %v\n", req, st, err)
	rep.Stat = st
	rep.Err = scs.setErr(err)
	return nil
}

func (scs *SigmaClntSrv) Create(ctx fs.CtxI, req scproto.SigmaCreateRequest, rep *scproto.SigmaFdReply) error {
	fd, err := scs.sc.Create(req.Path, sp.Tperm(req.Perm), sp.Tmode(req.Mode))
	db.DPrintf(db.SIGMACLNTSRV, "Create %v fd %v err %v\n", req, fd, err)
	rep.Fd = uint32(fd)
	rep.Err = scs.setErr(err)
	return nil
}

func (scs *SigmaClntSrv) Open(ctx fs.CtxI, req scproto.SigmaCreateRequest, rep *scproto.SigmaFdReply) error {
	fd, err := scs.sc.SigmaOS.Open(req.Path, sp.Tmode(req.Mode), sos.Twait(req.Wait))
	db.DPrintf(db.SIGMACLNTSRV, "Open %v fd %v err %v\n", req, fd, err)
	rep.Fd = uint32(fd)
	rep.Err = scs.setErr(err)
	return nil
}

func (scs *SigmaClntSrv) Rename(ctx fs.CtxI, req scproto.SigmaRenameRequest, rep *scproto.SigmaErrReply) error {
	err := scs.sc.Rename(req.Src, req.Dst)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "Rename %v %v\n", req, rep)
	return nil
}

func (scs *SigmaClntSrv) Remove(ctx fs.CtxI, req scproto.SigmaPathRequest, rep *scproto.SigmaErrReply) error {
	err := scs.sc.Remove(req.Path)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "Remove %v %v\n", req, rep)
	return nil
}

func (scs *SigmaClntSrv) GetFile(ctx fs.CtxI, req scproto.SigmaPathRequest, rep *scproto.SigmaDataReply) error {
	d, err := scs.sc.GetFile(req.Path)
	rep.Data = d
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "GetFile %v %v\n", req, rep)
	return nil
}

func (scs *SigmaClntSrv) PutFile(ctx fs.CtxI, req scproto.SigmaPutFileRequest, rep *scproto.SigmaSizeReply) error {
	db.DPrintf(db.SIGMACLNTSRV, "Invoke PutFile %v %v\n", req)
	sz, err := scs.sc.PutFile(req.Path, sp.Tperm(req.Perm), sp.Tmode(req.Mode), req.Data)
	rep.Size = uint64(sz)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "PutFile %v %v\n", req, rep)
	return nil
}

func (scs *SigmaClntSrv) Read(ctx fs.CtxI, req scproto.SigmaReadRequest, rep *scproto.SigmaDataReply) error {
	d, err := scs.sc.Read(int(req.Fd), sp.Tsize(req.Size))
	rep.Data = d
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "Read %v %v\n", req, rep)
	return nil
}

func (scs *SigmaClntSrv) Write(ctx fs.CtxI, req scproto.SigmaWriteRequest, rep *scproto.SigmaSizeReply) error {
	sz, err := scs.sc.Write(int(req.Fd), req.Data)
	rep.Size = uint64(sz)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "Write %v %v\n", req, rep)
	return nil
}

func (scs *SigmaClntSrv) Seek(ctx fs.CtxI, req scproto.SigmaSeekRequest, rep *scproto.SigmaErrReply) error {
	err := scs.sc.Seek(int(req.Fd), sp.Toffset(req.Offset))
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "Seek %v %v\n", req, rep)
	return nil
}

func (scs *SigmaClntSrv) WriteRead(ctx fs.CtxI, req scproto.SigmaWriteRequest, rep *scproto.SigmaDataReply) error {
	d, err := scs.sc.WriteRead(int(req.Fd), req.Data)
	db.DPrintf(db.SIGMACLNTSRV, "WriteRead %v %v %v\n", req, len(d), err)
	rep.Data = d
	rep.Err = scs.setErr(err)
	return nil
}

func (scs *SigmaClntSrv) CreateEphemeral(ctx fs.CtxI, req scproto.SigmaCreateRequest, rep *scproto.SigmaFdReply) error {
	fd, err := scs.sc.CreateEphemeral(req.Path, sp.Tperm(req.Perm), sp.Tmode(req.Mode), sp.TleaseId(req.LeaseId), req.Fence.Tfence())
	db.DPrintf(db.SIGMACLNTSRV, "CreateEphemeral %v %v %v\n", req, fd, err)
	rep.Fd = uint32(fd)
	rep.Err = scs.setErr(err)
	return nil
}

func (scs *SigmaClntSrv) ClntId(ctx fs.CtxI, req scproto.SigmaNullRequest, rep *scproto.SigmaClntIdReply) error {
	id := scs.sc.ClntId()
	rep.ClntId = uint64(id)
	db.DPrintf(db.SIGMACLNTSRV, "ClntId %v %v\n", req, rep)
	return nil
}

func (scs *SigmaClntSrv) FenceDir(ctx fs.CtxI, req scproto.SigmaFenceRequest, rep *scproto.SigmaErrReply) error {
	err := scs.sc.FenceDir(req.Path, req.Fence.Tfence())
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "FenceDir %v %v\n", req, rep)
	return nil
}

func (scs *SigmaClntSrv) WriteFence(ctx fs.CtxI, req scproto.SigmaWriteRequest, rep *scproto.SigmaSizeReply) error {
	sz, err := scs.sc.WriteFence(int(req.Fd), req.Data, req.Fence.Tfence())
	rep.Size = uint64(sz)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "WriteFence %v %v\n", req, rep)
	return nil
}

func (scs *SigmaClntSrv) MountTree(ctx fs.CtxI, req scproto.SigmaMountTreeRequest, rep *scproto.SigmaErrReply) error {
	err := scs.sc.MountTree(req.Addr, req.Tree, req.Mount)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "MountTree %v %v\n", req, rep)
	return nil
}

func (scs *SigmaClntSrv) PathLastMount(ctx fs.CtxI, req scproto.SigmaPathRequest, rep *scproto.SigmaLastMountReply) error {
	p1, p2, err := scs.sc.PathLastMount(req.Path)
	rep.Path1 = p1
	rep.Path2 = p2
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "PastLastMount %v %v\n", req, rep)
	return nil
}

func (scs *SigmaClntSrv) GetNamedMount(ctx fs.CtxI, req scproto.SigmaPathRequest, rep *scproto.SigmaMountReply) error {
	mnt := scs.sc.GetNamedMount()
	rep.Mount = mnt.TmountProto
	rep.Err = scs.setErr(nil)
	db.DPrintf(db.SIGMACLNTSRV, "PastLastMount %v %v\n", req, rep)
	return nil
}

func (scs *SigmaClntSrv) Disconnect(ctx fs.CtxI, req scproto.SigmaPathRequest, rep *scproto.SigmaErrReply) error {
	err := scs.sc.Disconnect(req.Path)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "Disconnect %v %v\n", req, rep)
	return nil
}

func (rpcch *RPCCh) ServeRequest(f []byte) ([]byte, *serr.Err) {
	b, err := rpcch.rpcs.WriteRead(rpcch.ctx, f)
	if err != nil {
		db.DPrintf(db.SIGMACLNTSRV, "serveRPC: writeRead err %v\n", err)
	}
	return b, err
}

func RunSigmaClntSrv(args []string) error {
	scs, err := NewSigmaClntSrv()
	if err != nil {
		return err
	}
	rpcs := rpcsrv.NewRPCSrv(scs, nil)
	rpcch := &RPCCh{nil, rpcs, ctx.NewCtxNull()}
	rpcch.dmx = demux.NewDemuxSrv(os.Stdin, os.Stdout, rpcch)
	ch := make(chan struct{})
	<-ch
	return nil
}
