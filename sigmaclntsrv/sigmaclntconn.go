package sigmaclntsrv

import (
	"bufio"
	"errors"
	"net"
	"sync"

	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/fidclnt"
	"sigmaos/fs"
	"sigmaos/proc"
	"sigmaos/rpcsrv"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
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

func newSigmaClntConn(conn net.Conn, pcfg *proc.ProcEnv, fidc *fidclnt.FidClnt) (*SigmaClntConn, error) {
	scs, err := NewSigmaClntSrvAPI(pcfg, fidc)
	//scs, err := NewSigmaClntSrvAPI(pcfg, nil)
	if err != nil {
		return nil, err
	}
	rpcs := rpcsrv.NewRPCSrv(scs, nil)
	scc := &SigmaClntConn{rpcs: rpcs, ctx: ctx.NewCtxNull(), conn: conn, api: scs}
	scc.dmx = demux.NewDemuxSrv(bufio.NewReaderSize(conn, sp.Conf.Conn.MSG_LEN),
		bufio.NewWriterSize(conn, sp.Conf.Conn.MSG_LEN), scc)
	db.DPrintf(db.SIGMACLNTSRV, "%v: newSigmaClntConn for %v\n", scs.sc.ClntId(), conn)
	return scc, nil
}

func (scc *SigmaClntConn) ServeRequest(f []byte) ([]byte, *serr.Err) {
	b, err := scc.rpcs.WriteRead(scc.ctx, f)
	if err != nil {
		db.DPrintf(db.SIGMACLNTSRV, "ServeRequest: writeRead err %v\n", err)
	}
	return b, err
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
	sc     *sigmaclnt.SigmaClnt
}

func (scc *SigmaClntSrvAPI) testAndSetClosed() bool {
	scc.mu.Lock()
	defer scc.mu.Unlock()
	b := scc.closed
	scc.closed = true
	return b
}

func NewSigmaClntSrvAPI(pcfg *proc.ProcEnv, fidc *fidclnt.FidClnt) (*SigmaClntSrvAPI, error) {
	sc, err := sigmaclnt.NewSigmaClntFsLibFidClnt(pcfg, fidc)
	if err != nil {
		return nil, err
	}

	scsa := &SigmaClntSrvAPI{sc: sc}
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

func (scs *SigmaClntSrvAPI) CloseFd(ctx fs.CtxI, req scproto.SigmaCloseRequest, rep *scproto.SigmaErrReply) error {
	err := scs.sc.CloseFd(int(req.Fd))
	db.DPrintf(db.SIGMACLNTSRV, "%v: CloseFd %v err %v\n", scs.sc.ClntId(), req, err)
	rep.Err = scs.setErr(err)
	return nil
}

func (scs *SigmaClntSrvAPI) Stat(ctx fs.CtxI, req scproto.SigmaPathRequest, rep *scproto.SigmaStatReply) error {
	st, err := scs.sc.Stat(req.Path)
	db.DPrintf(db.SIGMACLNTSRV, "%v: Stat %v st %v err %v\n", scs.sc.ClntId(), req, st, err)
	rep.Stat = st
	rep.Err = scs.setErr(err)
	return nil
}

func (scs *SigmaClntSrvAPI) Create(ctx fs.CtxI, req scproto.SigmaCreateRequest, rep *scproto.SigmaFdReply) error {
	fd, err := scs.sc.Create(req.Path, sp.Tperm(req.Perm), sp.Tmode(req.Mode))
	db.DPrintf(db.SIGMACLNTSRV, "%v: Create %v fd %v err %v\n", scs.sc.ClntId(), req, fd, err)
	rep.Fd = uint32(fd)
	rep.Err = scs.setErr(err)
	return nil
}

func (scs *SigmaClntSrvAPI) Open(ctx fs.CtxI, req scproto.SigmaCreateRequest, rep *scproto.SigmaFdReply) error {
	fd, err := scs.sc.SigmaOS.Open(req.Path, sp.Tmode(req.Mode), sos.Twait(req.Wait))
	db.DPrintf(db.SIGMACLNTSRV, "%v: Open %v fd %v err %v\n", scs.sc.ClntId(), req, fd, err)
	rep.Fd = uint32(fd)
	rep.Err = scs.setErr(err)
	return nil
}

func (scs *SigmaClntSrvAPI) Rename(ctx fs.CtxI, req scproto.SigmaRenameRequest, rep *scproto.SigmaErrReply) error {
	err := scs.sc.Rename(req.Src, req.Dst)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: Rename %v %v\n", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SigmaClntSrvAPI) Remove(ctx fs.CtxI, req scproto.SigmaPathRequest, rep *scproto.SigmaErrReply) error {
	err := scs.sc.Remove(req.Path)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: Remove %v %v\n", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SigmaClntSrvAPI) GetFile(ctx fs.CtxI, req scproto.SigmaPathRequest, rep *scproto.SigmaDataReply) error {
	d, err := scs.sc.GetFile(req.Path)
	rep.Data = d
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: GetFile %v %v\n", scs.sc.ClntId(), req, len(rep.Data))
	return nil
}

func (scs *SigmaClntSrvAPI) PutFile(ctx fs.CtxI, req scproto.SigmaPutFileRequest, rep *scproto.SigmaSizeReply) error {
	sz, err := scs.sc.SigmaOS.PutFile(req.Path, sp.Tperm(req.Perm), sp.Tmode(req.Mode), req.Data, sp.Toffset(req.Offset), sp.TleaseId(req.LeaseId))
	rep.Size = uint64(sz)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: PutFile %q %v %v\n", scs.sc.ClntId(), req.Path, len(req.Data), rep)
	return nil
}

func (scs *SigmaClntSrvAPI) Read(ctx fs.CtxI, req scproto.SigmaReadRequest, rep *scproto.SigmaDataReply) error {
	d, err := scs.sc.Read(int(req.Fd), sp.Tsize(req.Size))
	rep.Data = d
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: Read %v %v\n", scs.sc.ClntId(), req, len(rep.Data))
	return nil
}

func (scs *SigmaClntSrvAPI) Write(ctx fs.CtxI, req scproto.SigmaWriteRequest, rep *scproto.SigmaSizeReply) error {
	sz, err := scs.sc.Write(int(req.Fd), req.Data)
	rep.Size = uint64(sz)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: Write %v %v %v\n", scs.sc.ClntId(), req.Fd, len(req.Data), rep)
	return nil
}

func (scs *SigmaClntSrvAPI) Seek(ctx fs.CtxI, req scproto.SigmaSeekRequest, rep *scproto.SigmaErrReply) error {
	err := scs.sc.Seek(int(req.Fd), sp.Toffset(req.Offset))
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: Seek %v %v\n", req, rep)
	return nil
}

func (scs *SigmaClntSrvAPI) WriteRead(ctx fs.CtxI, req scproto.SigmaWriteRequest, rep *scproto.SigmaDataReply) error {
	d, err := scs.sc.WriteRead(int(req.Fd), req.Data)
	db.DPrintf(db.SIGMACLNTSRV, "%v: WriteRead %v %v %v %v\n", scs.sc.ClntId(), req.Fd, len(req.Data), len(d), err)
	rep.Data = d
	rep.Err = scs.setErr(err)
	return nil
}

func (scs *SigmaClntSrvAPI) CreateEphemeral(ctx fs.CtxI, req scproto.SigmaCreateRequest, rep *scproto.SigmaFdReply) error {
	fd, err := scs.sc.CreateEphemeral(req.Path, sp.Tperm(req.Perm), sp.Tmode(req.Mode), sp.TleaseId(req.LeaseId), req.Fence.Tfence())
	db.DPrintf(db.SIGMACLNTSRV, "%v: CreateEphemeral %v %v %v\n", scs.sc.ClntId(), req, fd, err)
	rep.Fd = uint32(fd)
	rep.Err = scs.setErr(err)
	return nil
}

func (scs *SigmaClntSrvAPI) ClntId(ctx fs.CtxI, req scproto.SigmaNullRequest, rep *scproto.SigmaClntIdReply) error {
	id := scs.sc.ClntId()
	rep.ClntId = uint64(id)
	rep.Err = scs.setErr(nil)
	db.DPrintf(db.SIGMACLNTSRV, "%v: ClntId %v %v\n", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SigmaClntSrvAPI) FenceDir(ctx fs.CtxI, req scproto.SigmaFenceRequest, rep *scproto.SigmaErrReply) error {
	err := scs.sc.FenceDir(req.Path, req.Fence.Tfence())
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: FenceDir %v %v\n", req, rep)
	return nil
}

func (scs *SigmaClntSrvAPI) WriteFence(ctx fs.CtxI, req scproto.SigmaWriteRequest, rep *scproto.SigmaSizeReply) error {
	sz, err := scs.sc.WriteFence(int(req.Fd), req.Data, req.Fence.Tfence())
	rep.Size = uint64(sz)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: WriteFence %v %v\n", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SigmaClntSrvAPI) DirWait(ctx fs.CtxI, req scproto.SigmaReadRequest, rep *scproto.SigmaErrReply) error {
	err := scs.sc.DirWait(int(req.Fd))
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: DirWait %v %v\n", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SigmaClntSrvAPI) IsLocalMount(ctx fs.CtxI, req scproto.SigmaMountRequest, rep *scproto.SigmaMountReply) error {
	ok, err := scs.sc.IsLocalMount(sp.Tmount{req.Mount})
	rep.Local = ok
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: IsLocalMount %v %v\n", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SigmaClntSrvAPI) MountTree(ctx fs.CtxI, req scproto.SigmaMountTreeRequest, rep *scproto.SigmaErrReply) error {
	err := scs.sc.MountTree(req.Addr, req.Tree, req.Mount)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: MountTree %v %v\n", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SigmaClntSrvAPI) PathLastMount(ctx fs.CtxI, req scproto.SigmaPathRequest, rep *scproto.SigmaLastMountReply) error {
	p1, p2, err := scs.sc.PathLastMount(req.Path)
	rep.Path1 = p1
	rep.Path2 = p2
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: PastLastMount %v %v\n", scs.sc.ClntId(), req, rep)
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
	db.DPrintf(db.SIGMACLNTSRV, "%v: PastLastMount %v %v\n", scs.sc.ClntId(), req, rep)
	return nil
}

// XXX need a few fslib instead of reusing bootkernel one?
func (scs *SigmaClntSrvAPI) NewRootMount(ctx fs.CtxI, req scproto.SigmaMountTreeRequest, rep *scproto.SigmaErrReply) error {
	err := scs.sc.NewRootMount(req.Tree, req.Mount)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: NewRootMount %v %v\n", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SigmaClntSrvAPI) Mounts(ctx fs.CtxI, req scproto.SigmaNullRequest, rep *scproto.SigmaMountsReply) error {
	mnts := scs.sc.Mounts()
	rep.Mounts = mnts
	rep.Err = scs.setErr(nil)
	db.DPrintf(db.SIGMACLNTSRV, "%v: Mounts %v %v\n", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SigmaClntSrvAPI) Detach(ctx fs.CtxI, req scproto.SigmaPathRequest, rep *scproto.SigmaErrReply) error {
	err := scs.sc.Detach(req.Path)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: Detach %v %v\n", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SigmaClntSrvAPI) Disconnect(ctx fs.CtxI, req scproto.SigmaPathRequest, rep *scproto.SigmaErrReply) error {
	err := scs.sc.Disconnect(req.Path)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: Disconnect %v %v\n", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SigmaClntSrvAPI) Close(ctx fs.CtxI, req scproto.SigmaNullRequest, rep *scproto.SigmaErrReply) error {
	db.DPrintf(db.ALWAYS, "%v: Close fslib %v\n", scs.sc.ClntId(), scs)
	var err error
	if !scs.testAndSetClosed() {
		err = scs.sc.Close()
	} else {
		err = nil
	}
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SIGMACLNTSRV, "%v: Close %v %v\n", scs.sc.ClntId(), req, rep)
	return nil
}
