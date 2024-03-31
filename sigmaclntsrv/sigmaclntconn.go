package sigmaclntsrv

import (
	"errors"
	"fmt"
	"net"
	"sync"

	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/fidclnt"
	"sigmaos/fs"
	"sigmaos/proc"
	rpcproto "sigmaos/rpc/proto"
	"sigmaos/rpcsrv"
	"sigmaos/serr"
	"sigmaos/sessp"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclntcodec"
	scproto "sigmaos/sigmaclntsrv/proto"
	sos "sigmaos/sigmaos"
	sp "sigmaos/sigmap"
)

// One SigmaClntConn per client connection
type SigmaClntConn struct {
	dmx  *demux.DemuxSrv
	rpcs *rpcsrv.RPCSrv
	ctx  fs.CtxI
	conn net.Conn
	api  *SigmaClntSrvAPI
}

func newSigmaClntConn(conn net.Conn, pe *proc.ProcEnv, fidc *fidclnt.FidClnt) (*SigmaClntConn, error) {
	scs, err := NewSigmaClntSrvAPI(pe, fidc)
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.ALWAYS, "Create SigmaClntConn with pe %v", pe)
	rpcs := rpcsrv.NewRPCSrv(scs, nil)
	scc := &SigmaClntConn{
		rpcs: rpcs,
		ctx:  ctx.NewCtxNull(),
		conn: conn,
		api:  scs,
	}
	iovm := demux.NewIoVecMap()
	scc.dmx = demux.NewDemuxSrv(scc, sigmaclntcodec.NewTransport(conn, iovm))
	return scc, nil
}

func (scc *SigmaClntConn) ServeRequest(c demux.CallI) (demux.CallI, *serr.Err) {
	req := c.(*sigmaclntcodec.Call)
	rep, err := scc.rpcs.WriteRead(scc.ctx, req.Iov)
	if err != nil {
		db.DPrintf(db.SIGMACLNTSRV, "ServeRequest: writeRead err %v", err)
	}
	return sigmaclntcodec.NewCall(req.Seqno, rep), nil
}

func (scc *SigmaClntConn) ReportError(err error) {
	db.DPrintf(db.DEMUXSRV, "ReportError err %v", err)
	go func() {
		scc.close()
	}()
}

func (scc *SigmaClntConn) close() error {
	if !scc.api.testAndSetClosed() {
		db.DPrintf(db.ALWAYS, "close: sigmaclntconn close %v", scc.api)
		scc.api.sc.Close()
	}
	if err := scc.conn.Close(); err != nil {
		return err
	}
	return scc.dmx.Close()
}

// SigmaClntSrvAPI exports the RPC methods that the server proxies.  The
// RPC methods correspond to the functions in the sigmaos interface.
type SigmaClntSrvAPI struct {
	mu     sync.Mutex
	closed bool
	fidc   *fidclnt.FidClnt
	sc     *sigmaclnt.SigmaClnt
}

func (scc *SigmaClntSrvAPI) testAndSetClosed() bool {
	scc.mu.Lock()
	defer scc.mu.Unlock()
	b := scc.closed
	scc.closed = true
	return b
}

func NewSigmaClntSrvAPI(pe *proc.ProcEnv, fidc *fidclnt.FidClnt) (*SigmaClntSrvAPI, error) {
	scsa := &SigmaClntSrvAPI{sc: nil, fidc: fidc}
	return scsa, nil
}

func (scs *SigmaClntSrvAPI) setErr(err error) *sp.Rerror {
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

func (scs *SigmaClntSrvAPI) Init(ctx fs.CtxI, req scproto.SigmaInitRequest, rep *scproto.SigmaErrReply) error {
	scs.mu.Lock()
	defer scs.mu.Unlock()

	if scs.sc != nil {
		err := fmt.Errorf("Error, re-init SigmaClntSrvAPI")
		rep.Err = scs.setErr(err)
		return err
	}
	pe := proc.NewProcEnvFromProto(req.ProcEnvProto)
	pe.UseSigmaclntd = false
	sc, err := sigmaclnt.NewSigmaClntFsLibFidClnt(pe, scs.fidc)
	if err != nil {
		rep.Err = scs.setErr(fmt.Errorf("Error init SigmaClntSrvAPI: %v pe %v", err, pe))
		return err
	}
	scs.sc = sc
	db.DPrintf(db.SIGMACLNTSRV, "%v: Init %v err %v", scs.sc.ClntId(), pe, err)
	rep.Err = scs.setErr(nil)
	return nil
}

func (scs *SigmaClntSrvAPI) CloseFd(ctx fs.CtxI, req scproto.SigmaCloseRequest, rep *scproto.SigmaErrReply) error {
	err := scs.sc.CloseFd(int(req.Fd))
	db.DPrintf(db.SIGMACLNTSRV, "%v: CloseFd %v err %v", scs.sc.ClntId(), req, err)
	rep.Err = scs.setErr(err)
	return nil
}

func (scs *SigmaClntSrvAPI) Stat(ctx fs.CtxI, req scproto.SigmaPathRequest, rep *scproto.SigmaStatReply) error {
	st, err := scs.sc.Stat(req.Path)
	db.DPrintf(db.SIGMACLNTSRV, "%v: Stat %v st %v err %v", scs.sc.ClntId(), req, st, err)
	rep.Stat = st
	rep.Err = scs.setErr(err)
	return nil
}

func (scs *SigmaClntSrvAPI) Create(ctx fs.CtxI, req scproto.SigmaCreateRequest, rep *scproto.SigmaFdReply) error {
	fd, err := scs.sc.Create(req.Path, sp.Tperm(req.Perm), sp.Tmode(req.Mode))
	db.DPrintf(db.SIGMACLNTSRV, "%v: Create %v fd %v err %v", scs.sc.ClntId(), req, fd, err)
	rep.Fd = uint32(fd)
	rep.Err = scs.setErr(err)
	return nil
}

func (scs *SigmaClntSrvAPI) Open(ctx fs.CtxI, req scproto.SigmaCreateRequest, rep *scproto.SigmaFdReply) error {
	fd, err := scs.sc.SigmaOS.Open(req.Path, sp.Tmode(req.Mode), sos.Twait(req.Wait))
	db.DPrintf(db.SIGMACLNTSRV, "%v: Open %v fd %v err %v", scs.sc.ClntId(), req, fd, err)
	rep.Fd = uint32(fd)
	rep.Err = scs.setErr(err)
	return nil
}

func (scs *SigmaClntSrvAPI) Rename(ctx fs.CtxI, req scproto.SigmaRenameRequest, rep *scproto.SigmaErrReply) error {
	err := scs.sc.Rename(req.Src, req.Dst)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: Rename %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SigmaClntSrvAPI) Remove(ctx fs.CtxI, req scproto.SigmaPathRequest, rep *scproto.SigmaErrReply) error {
	err := scs.sc.Remove(req.Path)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: Remove %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SigmaClntSrvAPI) GetFile(ctx fs.CtxI, req scproto.SigmaPathRequest, rep *scproto.SigmaDataReply) error {
	d, err := scs.sc.GetFile(req.Path)
	rep.Blob = &rpcproto.Blob{Iov: [][]byte{d}}
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: GetFile %v %v err %v", scs.sc.ClntId(), req, len(d), err)
	return nil
}

func (scs *SigmaClntSrvAPI) PutFile(ctx fs.CtxI, req scproto.SigmaPutFileRequest, rep *scproto.SigmaSizeReply) error {
	sz, err := scs.sc.SigmaOS.PutFile(req.Path, sp.Tperm(req.Perm), sp.Tmode(req.Mode), req.Blob.Iov[0], sp.Toffset(req.Offset), sp.TleaseId(req.LeaseId))
	rep.Size = uint64(sz)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: PutFile %q %v %v", scs.sc.ClntId(), req.Path, len(req.Blob.Iov), rep)
	return nil
}

func (scs *SigmaClntSrvAPI) Read(ctx fs.CtxI, req scproto.SigmaReadRequest, rep *scproto.SigmaDataReply) error {
	b := make([]byte, req.Size)
	cnt, err := scs.sc.Read(int(req.Fd), b)
	b = b[:cnt]
	rep.Blob = &rpcproto.Blob{Iov: [][]byte{b}}
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: Read %v %v size %v cnt %v len %v err %v", scs.sc.ClntId(), req.Size, req, len(rep.Blob.Iov), cnt, len(b), err)
	return nil
}

func (scs *SigmaClntSrvAPI) Write(ctx fs.CtxI, req scproto.SigmaWriteRequest, rep *scproto.SigmaSizeReply) error {
	db.DPrintf(db.SIGMACLNTSRV, "%v: Write sigmaclntsrv begin %v %v", scs.sc.ClntId(), req.Fd, len(req.Blob.Iov))
	sz, err := scs.sc.Write(int(req.Fd), req.Blob.Iov[0])
	rep.Size = uint64(sz)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: Write sigmaclntsrv returned %v %v %v err %v", scs.sc.ClntId(), req.Fd, len(req.Blob.Iov), rep, err)
	return nil
}

func (scs *SigmaClntSrvAPI) Seek(ctx fs.CtxI, req scproto.SigmaSeekRequest, rep *scproto.SigmaErrReply) error {
	err := scs.sc.Seek(int(req.Fd), sp.Toffset(req.Offset))
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: Seek %v %v", req, rep)
	return nil
}

func (scs *SigmaClntSrvAPI) WriteRead(ctx fs.CtxI, req scproto.SigmaWriteRequest, rep *scproto.SigmaDataReply) error {
	bl := make(sessp.IoVec, req.NOutVec)
	err := scs.sc.WriteRead(int(req.Fd), req.Blob.GetIoVec(), bl)
	db.DPrintf(db.SIGMACLNTSRV, "%v: WriteRead %v %v %v %v", scs.sc.ClntId(), req.Fd, len(req.Blob.Iov), len(bl), err)
	rep.Blob = rpcproto.NewBlob(bl)
	rep.Err = scs.setErr(err)
	return nil
}

func (scs *SigmaClntSrvAPI) CreateEphemeral(ctx fs.CtxI, req scproto.SigmaCreateRequest, rep *scproto.SigmaFdReply) error {
	fd, err := scs.sc.CreateEphemeral(req.Path, sp.Tperm(req.Perm), sp.Tmode(req.Mode), sp.TleaseId(req.LeaseId), req.Fence.Tfence())
	db.DPrintf(db.SIGMACLNTSRV, "%v: CreateEphemeral %v %v %v", scs.sc.ClntId(), req, fd, err)
	rep.Fd = uint32(fd)
	rep.Err = scs.setErr(err)
	return nil
}

func (scs *SigmaClntSrvAPI) ClntId(ctx fs.CtxI, req scproto.SigmaNullRequest, rep *scproto.SigmaClntIdReply) error {
	id := scs.sc.ClntId()
	rep.ClntId = uint64(id)
	rep.Err = scs.setErr(nil)
	db.DPrintf(db.SIGMACLNTSRV, "%v: ClntId %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SigmaClntSrvAPI) FenceDir(ctx fs.CtxI, req scproto.SigmaFenceRequest, rep *scproto.SigmaErrReply) error {
	err := scs.sc.FenceDir(req.Path, req.Fence.Tfence())
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: FenceDir %v %v", req, rep)
	return nil
}

func (scs *SigmaClntSrvAPI) WriteFence(ctx fs.CtxI, req scproto.SigmaWriteRequest, rep *scproto.SigmaSizeReply) error {
	sz, err := scs.sc.WriteFence(int(req.Fd), req.Blob.Iov[0], req.Fence.Tfence())
	rep.Size = uint64(sz)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: WriteFence %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SigmaClntSrvAPI) DirWait(ctx fs.CtxI, req scproto.SigmaReadRequest, rep *scproto.SigmaErrReply) error {
	err := scs.sc.DirWait(int(req.Fd))
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: DirWait %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SigmaClntSrvAPI) IsLocalMount(ctx fs.CtxI, req scproto.SigmaMountRequest, rep *scproto.SigmaMountReply) error {
	ok, err := scs.sc.IsLocalMount(sp.NewMountFromProto(req.Mount))
	rep.Local = ok
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: IsLocalMount %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SigmaClntSrvAPI) MountTree(ctx fs.CtxI, req scproto.SigmaMountTreeRequest, rep *scproto.SigmaErrReply) error {
	err := scs.sc.MountTree(sp.NewMountFromProto(req.Mount), req.Tree, req.MountName)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: MountTree %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SigmaClntSrvAPI) PathLastMount(ctx fs.CtxI, req scproto.SigmaPathRequest, rep *scproto.SigmaLastMountReply) error {
	p1, p2, err := scs.sc.PathLastMount(req.Path)
	rep.Path1 = p1
	rep.Path2 = p2
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: PastLastMount %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SigmaClntSrvAPI) GetNamedMount(ctx fs.CtxI, req scproto.SigmaPathRequest, rep *scproto.SigmaMountReply) error {
	mnt, err := scs.sc.GetNamedMount()
	if err != nil {
		db.DPrintf(db.ERROR, "Err GetNamedMount: %v", err)
		return err
	}
	rep.Mount = mnt.TmountProto
	rep.Err = scs.setErr(nil)
	db.DPrintf(db.SIGMACLNTSRV, "%v: PastLastMount %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

// XXX need a few fslib instead of reusing bootkernel one?
func (scs *SigmaClntSrvAPI) NewRootMount(ctx fs.CtxI, req scproto.SigmaMountTreeRequest, rep *scproto.SigmaErrReply) error {
	err := scs.sc.NewRootMount(req.Tree, req.MountName)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: NewRootMount %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SigmaClntSrvAPI) Mounts(ctx fs.CtxI, req scproto.SigmaNullRequest, rep *scproto.SigmaMountsReply) error {
	mnts := scs.sc.Mounts()
	rep.Mounts = mnts
	rep.Err = scs.setErr(nil)
	db.DPrintf(db.SIGMACLNTSRV, "%v: Mounts %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SigmaClntSrvAPI) Detach(ctx fs.CtxI, req scproto.SigmaPathRequest, rep *scproto.SigmaErrReply) error {
	err := scs.sc.Detach(req.Path)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: Detach %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SigmaClntSrvAPI) Disconnect(ctx fs.CtxI, req scproto.SigmaPathRequest, rep *scproto.SigmaErrReply) error {
	err := scs.sc.Disconnect(req.Path)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: Disconnect %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SigmaClntSrvAPI) Close(ctx fs.CtxI, req scproto.SigmaNullRequest, rep *scproto.SigmaErrReply) error {
	db.DPrintf(db.ALWAYS, "%v: Close fslib %v", scs.sc.ClntId(), scs)
	var err error
	if !scs.testAndSetClosed() {
		err = scs.sc.Close()
	} else {
		err = nil
	}
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: Close %v %v", scs.sc.ClntId(), req, rep)
	return nil
}
