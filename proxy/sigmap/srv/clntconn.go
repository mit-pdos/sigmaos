package srv

import (
	"errors"
	"fmt"
	"net"
	"sync"

	"sigmaos/api/fs"
	sos "sigmaos/api/sigmaos"
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/proc"
	scproto "sigmaos/proxy/sigmap/proto"
	"sigmaos/proxy/sigmap/transport"
	rpcproto "sigmaos/rpc/proto"
	rpcsrv "sigmaos/rpc/srv"
	"sigmaos/serr"
	sessp "sigmaos/session/proto"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/fidclnt"
	sp "sigmaos/sigmap"
	"sigmaos/util/io/demux"
)

// One SigmaClntConn per client connection
type SigmaClntConn struct {
	dmx  *demux.DemuxSrv
	rpcs *rpcsrv.RPCSrv
	ctx  fs.CtxI
	conn net.Conn
	api  *SPProxySrvAPI
}

func newSigmaClntConn(conn net.Conn, pe *proc.ProcEnv, fidc *fidclnt.FidClnt) (*SigmaClntConn, error) {
	scs, err := NewSPProxySrvAPI(pe, fidc)
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
	scc.dmx = demux.NewDemuxSrv(scc, transport.NewTransport(conn, iovm))
	return scc, nil
}

func (scc *SigmaClntConn) ServeRequest(c demux.CallI) (demux.CallI, *serr.Err) {
	req := c.(*transport.Call)
	rep, err := scc.rpcs.WriteRead(scc.ctx, req.Iov)
	if err != nil {
		db.DPrintf(db.SPPROXYSRV, "ServeRequest: writeRead err %v", err)
	}
	return transport.NewCall(req.Seqno, rep), nil
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

// SPProxySrvAPI exports the RPC methods that the server proxies.  The
// RPC methods correspond to the functions in the sigmaos interface.
type SPProxySrvAPI struct {
	mu          sync.Mutex
	closed      bool
	hasProcClnt bool
	fidc        *fidclnt.FidClnt
	sc          *sigmaclnt.SigmaClnt
}

func (scc *SPProxySrvAPI) testAndSetClosed() bool {
	scc.mu.Lock()
	defer scc.mu.Unlock()
	b := scc.closed
	scc.closed = true
	return b
}

func NewSPProxySrvAPI(pe *proc.ProcEnv, fidc *fidclnt.FidClnt) (*SPProxySrvAPI, error) {
	scsa := &SPProxySrvAPI{sc: nil, fidc: fidc}
	return scsa, nil
}

func (scs *SPProxySrvAPI) setErr(err error) *sp.Rerror {
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

func (scs *SPProxySrvAPI) Init(ctx fs.CtxI, req scproto.SigmaInitReq, rep *scproto.SigmaErrRep) error {
	scs.mu.Lock()
	defer scs.mu.Unlock()

	if scs.sc != nil {
		err := fmt.Errorf("Error, re-init SPProxySrvAPI")
		rep.Err = scs.setErr(err)
		return err
	}
	pe := proc.NewProcEnvFromProto(req.ProcEnvProto)
	pe.UseSPProxy = false
	pe.UseDialProxy = false
	db.DPrintf(db.SPPROXYSRV, "Init pe %v", pe)
	sc, err := sigmaclnt.NewSigmaClntFsLibFidClnt(pe, scs.fidc)
	if err != nil {
		rep.Err = scs.setErr(fmt.Errorf("Error init SPProxySrvAPI: %v pe %v", err, pe))
		return err
	}
	scs.sc = sc
	db.DPrintf(db.SPPROXYSRV, "%v: Init %v err %v", scs.sc.ClntId(), pe, err)
	rep.Err = scs.setErr(nil)
	return nil
}

func (scs *SPProxySrvAPI) CloseFd(ctx fs.CtxI, req scproto.SigmaCloseReq, rep *scproto.SigmaErrRep) error {
	err := scs.sc.CloseFd(int(req.Fd))
	db.DPrintf(db.SPPROXYSRV, "%v: CloseFd %v err %v", scs.sc.ClntId(), req, err)
	rep.Err = scs.setErr(err)
	return nil
}

func (scs *SPProxySrvAPI) Stat(ctx fs.CtxI, req scproto.SigmaPathReq, rep *scproto.SigmaStatRep) error {
	st, err := scs.sc.Stat(req.Path)
	db.DPrintf(db.SPPROXYSRV, "%v: Stat %v st %v err %v", scs.sc.ClntId(), req, st, err)
	rep.Stat = st.StatProto()
	rep.Err = scs.setErr(err)
	return nil
}

func (scs *SPProxySrvAPI) Create(ctx fs.CtxI, req scproto.SigmaCreateReq, rep *scproto.SigmaFdRep) error {
	fd, err := scs.sc.Create(req.Path, sp.Tperm(req.Perm), sp.Tmode(req.Mode))
	db.DPrintf(db.SPPROXYSRV, "%v: Create %v fd %v err %v", scs.sc.ClntId(), req, fd, err)
	rep.Fd = uint32(fd)
	rep.Err = scs.setErr(err)
	return nil
}

func (scs *SPProxySrvAPI) Open(ctx fs.CtxI, req scproto.SigmaCreateReq, rep *scproto.SigmaFdRep) error {
	fd, err := scs.sc.FileAPI.Open(req.Path, sp.Tmode(req.Mode), sos.Twait(req.Wait))
	db.DPrintf(db.SPPROXYSRV, "%v: Open %v fd %v err %v", scs.sc.ClntId(), req, fd, err)
	rep.Fd = uint32(fd)
	rep.Err = scs.setErr(err)
	return nil
}

func (scs *SPProxySrvAPI) Rename(ctx fs.CtxI, req scproto.SigmaRenameReq, rep *scproto.SigmaErrRep) error {
	err := scs.sc.Rename(req.Src, req.Dst)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: Rename %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SPProxySrvAPI) Remove(ctx fs.CtxI, req scproto.SigmaPathReq, rep *scproto.SigmaErrRep) error {
	err := scs.sc.Remove(req.Path)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: Remove %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SPProxySrvAPI) GetFile(ctx fs.CtxI, req scproto.SigmaPathReq, rep *scproto.SigmaDataRep) error {
	d, err := scs.sc.GetFile(req.Path)
	rep.Blob = &rpcproto.Blob{Iov: [][]byte{d}}
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: GetFile %v %v err %v", scs.sc.ClntId(), req, len(d), err)
	return nil
}

func (scs *SPProxySrvAPI) PutFile(ctx fs.CtxI, req scproto.SigmaPutFileReq, rep *scproto.SigmaSizeRep) error {
	sz, err := scs.sc.FileAPI.PutFile(req.Path, sp.Tperm(req.Perm), sp.Tmode(req.Mode), req.Blob.Iov[0], sp.Toffset(req.Offset), sp.TleaseId(req.LeaseId))
	rep.Size = uint64(sz)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: PutFile %q %v %v", scs.sc.ClntId(), req.Path, len(req.Blob.Iov), rep)
	return nil
}

func (scs *SPProxySrvAPI) Read(ctx fs.CtxI, req scproto.SigmaReadReq, rep *scproto.SigmaDataRep) error {
	b := make([]byte, req.Size)
	o := sp.Toffset(req.Off)
	var cnt sp.Tsize
	var err error
	if o == sp.NoOffset {
		cnt, err = scs.sc.Read(int(req.Fd), b)
	} else {
		cnt, err = scs.sc.Pread(int(req.Fd), b, o)
	}
	b = b[:cnt]
	rep.Blob = &rpcproto.Blob{Iov: [][]byte{b}}
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: Read %v %v size %v cnt %v len %v err %v", scs.sc.ClntId(), req.Size, req, len(rep.Blob.Iov), cnt, len(b), err)
	return nil
}

func (scs *SPProxySrvAPI) Write(ctx fs.CtxI, req scproto.SigmaWriteReq, rep *scproto.SigmaSizeRep) error {
	db.DPrintf(db.SPPROXYSRV, "%v: Write spproxysrv begin %v %v", scs.sc.ClntId(), req.Fd, len(req.Blob.Iov))
	sz, err := scs.sc.Write(int(req.Fd), req.Blob.Iov[0])
	rep.Size = uint64(sz)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: Write spproxysrv returned %v %v %v err %v", scs.sc.ClntId(), req.Fd, len(req.Blob.Iov), rep, err)
	return nil
}

func (scs *SPProxySrvAPI) Seek(ctx fs.CtxI, req scproto.SigmaSeekReq, rep *scproto.SigmaErrRep) error {
	err := scs.sc.Seek(int(req.Fd), sp.Toffset(req.Offset))
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: Seek %v %v", req, rep)
	return nil
}

func (scs *SPProxySrvAPI) WriteRead(ctx fs.CtxI, req scproto.SigmaWriteReq, rep *scproto.SigmaDataRep) error {
	bl := make(sessp.IoVec, req.NOutVec)
	err := scs.sc.WriteRead(int(req.Fd), req.Blob.GetIoVec(), bl)
	db.DPrintf(db.SPPROXYSRV, "%v: WriteRead %v %v %v %v", scs.sc.ClntId(), req.Fd, len(req.Blob.Iov), len(bl), err)
	rep.Blob = rpcproto.NewBlob(bl)
	rep.Err = scs.setErr(err)
	return nil
}

func (scs *SPProxySrvAPI) CreateLeased(ctx fs.CtxI, req scproto.SigmaCreateReq, rep *scproto.SigmaFdRep) error {
	fd, err := scs.sc.CreateLeased(req.Path, sp.Tperm(req.Perm), sp.Tmode(req.Mode), sp.TleaseId(req.LeaseId), req.Fence.Tfence())
	db.DPrintf(db.SPPROXYSRV, "%v: CreateLeased %v %v %v", scs.sc.ClntId(), req, fd, err)
	rep.Fd = uint32(fd)
	rep.Err = scs.setErr(err)
	return nil
}

func (scs *SPProxySrvAPI) ClntId(ctx fs.CtxI, req scproto.SigmaNullReq, rep *scproto.SigmaClntIdRep) error {
	id := scs.sc.ClntId()
	rep.ClntId = uint64(id)
	rep.Err = scs.setErr(nil)
	db.DPrintf(db.SPPROXYSRV, "%v: ClntId %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SPProxySrvAPI) FenceDir(ctx fs.CtxI, req scproto.SigmaFenceReq, rep *scproto.SigmaErrRep) error {
	err := scs.sc.FenceDir(req.Path, req.Fence.Tfence())
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: FenceDir %v %v", req, rep)
	return nil
}

func (scs *SPProxySrvAPI) WriteFence(ctx fs.CtxI, req scproto.SigmaWriteReq, rep *scproto.SigmaSizeRep) error {
	sz, err := scs.sc.WriteFence(int(req.Fd), req.Blob.Iov[0], req.Fence.Tfence())
	rep.Size = uint64(sz)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: WriteFence %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SPProxySrvAPI) DirWatch(ctx fs.CtxI, req scproto.SigmaReadReq, rep *scproto.SigmaFdRep) error {
	fd, err := scs.sc.DirWatch(int(req.Fd))
	rep.Fd = uint32(fd)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: DirWatch %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SPProxySrvAPI) IsLocalMount(ctx fs.CtxI, req scproto.SigmaMountReq, rep *scproto.SigmaMountRep) error {
	ok, err := scs.sc.IsLocalMount(sp.NewEndpointFromProto(req.Endpoint))
	rep.Local = ok
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: IsLocalMount %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SPProxySrvAPI) MountTree(ctx fs.CtxI, req scproto.SigmaMountTreeReq, rep *scproto.SigmaErrRep) error {
	err := scs.sc.MountTree(sp.NewEndpointFromProto(req.Endpoint), req.Tree, req.MountName)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: MountTree %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SPProxySrvAPI) PathLastMount(ctx fs.CtxI, req scproto.SigmaPathReq, rep *scproto.SigmaLastMountRep) error {
	p1, p2, err := scs.sc.PathLastMount(req.Path)
	rep.Path1 = p1
	rep.Path2 = p2
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: PastLastMount %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SPProxySrvAPI) InvalidateNamedEndpointCacheEntryRealm(ctx fs.CtxI, req scproto.SigmaRealmReq, rep *scproto.SigmaMountRep) error {
	err := scs.sc.InvalidateNamedEndpointCacheEntryRealm(sp.Trealm(req.RealmStr))
	if err != nil {
		db.DPrintf(db.ERROR, "Err GetNamedEndpoint: %v", err)
		return err
	}
	rep.Err = scs.setErr(nil)
	db.DPrintf(db.SPPROXYSRV, "%v: PastLastMount %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SPProxySrvAPI) GetNamedEndpoint(ctx fs.CtxI, req scproto.SigmaRealmReq, rep *scproto.SigmaMountRep) error {
	ep, err := scs.sc.GetNamedEndpointRealm(sp.Trealm(req.RealmStr))
	if err != nil {
		db.DPrintf(db.ERROR, "Err GetNamedEndpoint: %v", err)
		return err
	}
	rep.Endpoint = ep.TendpointProto
	rep.Err = scs.setErr(nil)
	db.DPrintf(db.SPPROXYSRV, "%v: PastLastMount %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SPProxySrvAPI) NewRootMount(ctx fs.CtxI, req scproto.SigmaMountTreeReq, rep *scproto.SigmaErrRep) error {
	err := scs.sc.NewRootMount(req.Tree, req.MountName)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: NewRootMount %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SPProxySrvAPI) Mounts(ctx fs.CtxI, req scproto.SigmaNullReq, rep *scproto.SigmaMountsRep) error {
	mnts := scs.sc.Mounts()
	rep.Endpoints = mnts
	rep.Err = scs.setErr(nil)
	db.DPrintf(db.SPPROXYSRV, "%v: Mounts %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SPProxySrvAPI) Detach(ctx fs.CtxI, req scproto.SigmaPathReq, rep *scproto.SigmaErrRep) error {
	err := scs.sc.Detach(req.Path)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: Detach %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SPProxySrvAPI) Disconnect(ctx fs.CtxI, req scproto.SigmaPathReq, rep *scproto.SigmaErrRep) error {
	err := scs.sc.Disconnect(req.Path)
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: Disconnect %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SPProxySrvAPI) Close(ctx fs.CtxI, req scproto.SigmaNullReq, rep *scproto.SigmaErrRep) error {
	db.DPrintf(db.ALWAYS, "%v: Close fslib %v", scs.sc.ClntId(), scs)
	var err error
	if !scs.testAndSetClosed() {
		err = scs.sc.Close()
	} else {
		err = nil
	}
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: Close %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

// ========== Procclnt API ==========
func (scs *SPProxySrvAPI) initProcClnt() error {
	scs.mu.Lock()
	defer scs.mu.Unlock()

	var err error
	// Make a procclnt if we haven't already
	if !scs.hasProcClnt {
		scs.hasProcClnt = true
		err = scs.sc.NewProcClnt()
	}
	// If unsuccessful, bail out
	if err != nil {
		db.DPrintf(db.SPPROXYSRV_ERR, "%v: Failed to create procclnt: %v", scs.sc.ClntId(), err)
	}
	return err
}

func (scs *SPProxySrvAPI) Started(ctx fs.CtxI, req scproto.SigmaNullReq, rep *scproto.SigmaErrRep) error {
	db.DPrintf(db.SPPROXYSRV, "%v: Started", scs.sc.ClntId())
	err := scs.initProcClnt()
	if err != nil {
		rep.Err = scs.setErr(err)
		return nil
	}
	err = scs.sc.Started()
	if err != nil {
		db.DPrintf(db.SPPROXYSRV_ERR, "%v: Started err: %v", scs.sc.ClntId(), err)
	}
	rep.Err = scs.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: Started done %v %v", scs.sc.ClntId(), req, rep)
	return nil
}

func (scs *SPProxySrvAPI) Exited(ctx fs.CtxI, req scproto.SigmaExitedReq, rep *scproto.SigmaErrRep) error {
	status := proc.Tstatus(req.Status)
	db.DPrintf(db.SPPROXYSRV, "%v: Exited %v", scs.sc.ClntId(), status, req.Msg)
	err := scs.initProcClnt()
	rep.Err = scs.setErr(err)
	if err != nil {
		return nil
	}
	scs.sc.Exited(proc.NewStatusInfo(proc.StatusOK, req.Msg, nil))
	db.DPrintf(db.SPPROXYSRV, "%v: Exited done %v %v", scs.sc.ClntId(), req, rep)
	return nil
}
