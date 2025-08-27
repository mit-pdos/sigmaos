package srv

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"sigmaos/api/fs"
	sos "sigmaos/api/sigmaos"
	epcacheclnt "sigmaos/apps/epcache/clnt"
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/proc"
	scproto "sigmaos/proxy/sigmap/proto"
	rpcproto "sigmaos/rpc/proto"
	rpcsrv "sigmaos/rpc/srv"
	"sigmaos/rpc/transport"
	"sigmaos/serr"
	sessp "sigmaos/session/proto"
	"sigmaos/sigmaclnt"
	"sigmaos/sigmaclnt/fidclnt"
	sp "sigmaos/sigmap"
	"sigmaos/util/io/demux"
	"sigmaos/util/perf"
)

// One SigmaClntConn per client connection
type SigmaClntConn struct {
	dmx  *demux.DemuxSrv
	rpcs *rpcsrv.RPCSrv
	ctx  fs.CtxI
	conn net.Conn
	api  *SPProxySrvAPI
	spps *SPProxySrv
}

func newSigmaClntConn(spps *SPProxySrv, conn net.Conn, pe *proc.ProcEnv, fidc *fidclnt.FidClnt) (*SigmaClntConn, error) {
	scapi, err := NewSPProxySrvAPI(spps, pe, fidc)
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.SPPROXYSRV, "Create SigmaClntConn with pe %v", pe)
	rpcs := rpcsrv.NewRPCSrv(scapi, nil)
	scc := &SigmaClntConn{
		rpcs: rpcs,
		ctx:  ctx.NewCtxNull(),
		conn: conn,
		api:  scapi,
		spps: spps,
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
	mu                  sync.Mutex
	cond                *sync.Cond
	closed              bool
	procClntInitStarted bool
	procClntInitDone    bool
	procClntInitErr     error
	fidc                *fidclnt.FidClnt
	sc                  *sigmaclnt.SigmaClnt
	epcc                *epcacheclnt.EndpointCacheClnt
	spps                *SPProxySrv
}

func (scc *SPProxySrvAPI) testAndSetClosed() bool {
	scc.mu.Lock()
	defer scc.mu.Unlock()
	b := scc.closed
	scc.closed = true
	return b
}

func NewSPProxySrvAPI(spps *SPProxySrv, pe *proc.ProcEnv, fidc *fidclnt.FidClnt) (*SPProxySrvAPI, error) {
	sca := &SPProxySrvAPI{
		sc:   nil,
		fidc: fidc,
		spps: spps,
	}
	sca.cond = sync.NewCond(&sca.mu)
	return sca, nil
}

func (sca *SPProxySrvAPI) setErr(err error) *sp.Rerror {
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

func (sca *SPProxySrvAPI) Init(ctx fs.CtxI, req scproto.SigmaInitReq, rep *scproto.SigmaErrRep) error {
	sca.mu.Lock()
	defer sca.mu.Unlock()

	if sca.sc != nil {
		err := fmt.Errorf("Error, re-init SPProxySrvAPI")
		rep.Err = sca.setErr(err)
		return err
	}
	pe := proc.NewProcEnvFromProto(req.ProcEnvProto)
	db.DPrintf(db.SPPROXYSRV, "Init pe %v", pe)
	start := time.Now()
	// Don't create a procclnt for the test program
	sc, epcc, err := sca.spps.getSigmaClnt(pe, nil)
	if err != nil {
		db.DPrintf(db.SPPROXYSRV_ERR, "Error init SPProxySrvAPI: %v pe %v", err, pe)
		rep.Err = sca.setErr(fmt.Errorf("Error init SPProxySrvAPI: %v pe %v", err, pe))
		return err
	}
	perf.LogSpawnLatency("SPProxySrv.Init wait getOrCreateSigmaClnt", pe.GetPID(), pe.GetSpawnTime(), start)
	sca.sc = sc
	sca.epcc = epcc
	db.DPrintf(db.SPPROXYSRV, "%v: Init done %v", sca.sc.ClntId(), pe)
	rep.Err = sca.setErr(nil)
	return nil
}

func (sca *SPProxySrvAPI) CloseFd(ctx fs.CtxI, req scproto.SigmaCloseReq, rep *scproto.SigmaErrRep) error {
	err := sca.sc.CloseFd(int(req.Fd))
	db.DPrintf(db.SPPROXYSRV, "%v: CloseFd %v err %v", sca.sc.ClntId(), req, err)
	rep.Err = sca.setErr(err)
	return nil
}

func (sca *SPProxySrvAPI) Stat(ctx fs.CtxI, req scproto.SigmaPathReq, rep *scproto.SigmaStatRep) error {
	st, err := sca.sc.Stat(req.Path)
	db.DPrintf(db.SPPROXYSRV, "%v: Stat %v st %v err %v", sca.sc.ClntId(), req, st, err)
	rep.Stat = st.StatProto()
	rep.Err = sca.setErr(err)
	return nil
}

func (sca *SPProxySrvAPI) Create(ctx fs.CtxI, req scproto.SigmaCreateReq, rep *scproto.SigmaFdRep) error {
	fd, err := sca.sc.Create(req.Path, sp.Tperm(req.Perm), sp.Tmode(req.Mode))
	db.DPrintf(db.SPPROXYSRV, "%v: Create %v fd %v err %v", sca.sc.ClntId(), req, fd, err)
	rep.Fd = uint32(fd)
	rep.Err = sca.setErr(err)
	return nil
}

func (sca *SPProxySrvAPI) Open(ctx fs.CtxI, req scproto.SigmaCreateReq, rep *scproto.SigmaFdRep) error {
	fd, err := sca.sc.FileAPI.Open(req.Path, sp.Tmode(req.Mode), sos.Twait(req.Wait))
	db.DPrintf(db.SPPROXYSRV, "%v: Open %v fd %v err %v", sca.sc.ClntId(), req, fd, err)
	rep.Fd = uint32(fd)
	rep.Err = sca.setErr(err)
	return nil
}

func (sca *SPProxySrvAPI) Rename(ctx fs.CtxI, req scproto.SigmaRenameReq, rep *scproto.SigmaErrRep) error {
	err := sca.sc.Rename(req.Src, req.Dst)
	rep.Err = sca.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: Rename %v %v", sca.sc.ClntId(), req, rep)
	return nil
}

func (sca *SPProxySrvAPI) Remove(ctx fs.CtxI, req scproto.SigmaPathReq, rep *scproto.SigmaErrRep) error {
	err := sca.sc.Remove(req.Path)
	rep.Err = sca.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: Remove %v %v", sca.sc.ClntId(), req, rep)
	return nil
}

func (sca *SPProxySrvAPI) GetFile(ctx fs.CtxI, req scproto.SigmaPathReq, rep *scproto.SigmaDataRep) error {
	db.DPrintf(db.SPPROXYSRV, "%v: GetFile %v", sca.sc.ClntId(), req)
	d, err := sca.sc.GetFile(req.Path)
	rep.Blob = &rpcproto.Blob{Iov: [][]byte{d}}
	rep.Err = sca.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: GetFile %v %v err %v", sca.sc.ClntId(), req, len(d), err)
	return nil
}

func (sca *SPProxySrvAPI) PutFile(ctx fs.CtxI, req scproto.SigmaPutFileReq, rep *scproto.SigmaSizeRep) error {
	db.DPrintf(db.SPPROXYSRV, "%v: PutFile req %v %v", sca.sc.ClntId(), req.Path, len(req.Blob.Iov))
	sz, err := sca.sc.FileAPI.PutFile(req.Path, sp.Tperm(req.Perm), sp.Tmode(req.Mode), req.Blob.Iov[0], sp.Toffset(req.Offset), sp.TleaseId(req.LeaseId))
	rep.Size = uint64(sz)
	rep.Err = sca.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: PutFile %q %v %v", sca.sc.ClntId(), req.Path, len(req.Blob.Iov), rep)
	return nil
}

func (sca *SPProxySrvAPI) Read(ctx fs.CtxI, req scproto.SigmaReadReq, rep *scproto.SigmaDataRep) error {
	b := make([]byte, req.Size)
	o := sp.Toffset(req.Off)
	var cnt sp.Tsize
	var err error
	if o == sp.NoOffset {
		cnt, err = sca.sc.Read(int(req.Fd), b)
	} else {
		cnt, err = sca.sc.Pread(int(req.Fd), b, o)
	}
	b = b[:cnt]
	rep.Blob = &rpcproto.Blob{Iov: [][]byte{b}}
	rep.Err = sca.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: Read %v %v size %v cnt %v len %v err %v", sca.sc.ClntId(), req.Size, req, len(rep.Blob.Iov), cnt, len(b), err)
	return nil
}

func (sca *SPProxySrvAPI) Write(ctx fs.CtxI, req scproto.SigmaWriteReq, rep *scproto.SigmaSizeRep) error {
	db.DPrintf(db.SPPROXYSRV, "%v: Write spproxysrv begin %v %v", sca.sc.ClntId(), req.Fd, len(req.Blob.Iov))
	sz, err := sca.sc.Write(int(req.Fd), req.Blob.Iov[0])
	rep.Size = uint64(sz)
	rep.Err = sca.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: Write spproxysrv returned %v %v %v err %v", sca.sc.ClntId(), req.Fd, len(req.Blob.Iov), rep, err)
	return nil
}

func (sca *SPProxySrvAPI) Seek(ctx fs.CtxI, req scproto.SigmaSeekReq, rep *scproto.SigmaErrRep) error {
	err := sca.sc.Seek(int(req.Fd), sp.Toffset(req.Offset))
	rep.Err = sca.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: Seek %v %v", req, rep)
	return nil
}

func (sca *SPProxySrvAPI) WriteRead(ctx fs.CtxI, req scproto.SigmaWriteReq, rep *scproto.SigmaDataRep) error {
	bl := make(sessp.IoVec, req.NOutVec)
	start := time.Now()
	err := sca.sc.WriteRead(int(req.Fd), req.Blob.GetIoVec(), bl)
	db.DPrintf(db.SPPROXYSRV, "%v: WriteRead (lat=%v) fd:%v nInIOV:%v nOutIOV:%v err:%v", sca.sc.ClntId(), time.Since(start), req.Fd, len(req.Blob.Iov), len(bl), err)
	rep.Blob = rpcproto.NewBlob(bl)
	rep.Err = sca.setErr(err)
	return nil
}

func (sca *SPProxySrvAPI) CreateLeased(ctx fs.CtxI, req scproto.SigmaCreateReq, rep *scproto.SigmaFdRep) error {
	fd, err := sca.sc.CreateLeased(req.Path, sp.Tperm(req.Perm), sp.Tmode(req.Mode), sp.TleaseId(req.LeaseId), req.Fence.Tfence())
	db.DPrintf(db.SPPROXYSRV, "%v: CreateLeased %v %v %v", sca.sc.ClntId(), req, fd, err)
	rep.Fd = uint32(fd)
	rep.Err = sca.setErr(err)
	return nil
}

func (sca *SPProxySrvAPI) ClntId(ctx fs.CtxI, req scproto.SigmaNullReq, rep *scproto.SigmaClntIdRep) error {
	start := time.Now()
	id := sca.sc.ClntId()
	rep.ClntId = uint64(id)
	rep.Err = sca.setErr(nil)
	db.DPrintf(db.SPPROXYSRV, "%v: ClntId (lat=%v) %v %v", sca.sc.ClntId(), time.Since(start), req, rep)
	return nil
}

func (sca *SPProxySrvAPI) FenceDir(ctx fs.CtxI, req scproto.SigmaFenceReq, rep *scproto.SigmaErrRep) error {
	err := sca.sc.FenceDir(req.Path, req.Fence.Tfence())
	rep.Err = sca.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: FenceDir %v %v", req, rep)
	return nil
}

func (sca *SPProxySrvAPI) WriteFence(ctx fs.CtxI, req scproto.SigmaWriteReq, rep *scproto.SigmaSizeRep) error {
	sz, err := sca.sc.WriteFence(int(req.Fd), req.Blob.Iov[0], req.Fence.Tfence())
	rep.Size = uint64(sz)
	rep.Err = sca.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: WriteFence %v %v", sca.sc.ClntId(), req, rep)
	return nil
}

func (sca *SPProxySrvAPI) DirWatch(ctx fs.CtxI, req scproto.SigmaReadReq, rep *scproto.SigmaFdRep) error {
	fd, err := sca.sc.DirWatch(int(req.Fd))
	rep.Fd = uint32(fd)
	rep.Err = sca.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: DirWatch %v %v", sca.sc.ClntId(), req, rep)
	return nil
}

func (sca *SPProxySrvAPI) IsLocalMount(ctx fs.CtxI, req scproto.SigmaMountReq, rep *scproto.SigmaMountRep) error {
	ok, err := sca.sc.IsLocalMount(sp.NewEndpointFromProto(req.Endpoint))
	rep.Local = ok
	rep.Err = sca.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: IsLocalMount %v %v", sca.sc.ClntId(), req, rep)
	return nil
}

func (sca *SPProxySrvAPI) MountTree(ctx fs.CtxI, req scproto.SigmaMountTreeReq, rep *scproto.SigmaErrRep) error {
	err := sca.sc.MountTree(sp.NewEndpointFromProto(req.Endpoint), req.Tree, req.MountName)
	rep.Err = sca.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: MountTree %v %v", sca.sc.ClntId(), req, rep)
	return nil
}

func (sca *SPProxySrvAPI) PathLastMount(ctx fs.CtxI, req scproto.SigmaPathReq, rep *scproto.SigmaLastMountRep) error {
	p1, p2, err := sca.sc.PathLastMount(req.Path)
	rep.Path1 = p1
	rep.Path2 = p2
	rep.Err = sca.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: PastLastMount %v %v", sca.sc.ClntId(), req, rep)
	return nil
}

func (sca *SPProxySrvAPI) InvalidateNamedEndpointCacheEntryRealm(ctx fs.CtxI, req scproto.SigmaRealmReq, rep *scproto.SigmaMountRep) error {
	err := sca.sc.InvalidateNamedEndpointCacheEntryRealm(sp.Trealm(req.RealmStr))
	if err != nil {
		db.DPrintf(db.ERROR, "Err GetNamedEndpoint: %v", err)
		return err
	}
	rep.Err = sca.setErr(nil)
	db.DPrintf(db.SPPROXYSRV, "%v: PastLastMount %v %v", sca.sc.ClntId(), req, rep)
	return nil
}

func (sca *SPProxySrvAPI) GetNamedEndpointRealm(ctx fs.CtxI, req scproto.SigmaRealmReq, rep *scproto.SigmaMountRep) error {
	ep, err := sca.sc.GetNamedEndpointRealm(sp.Trealm(req.RealmStr))
	if err != nil {
		db.DPrintf(db.ERROR, "Err GetNamedEndpoint: %v", err)
		return err
	}
	rep.Endpoint = ep.TendpointProto
	rep.Err = sca.setErr(nil)
	db.DPrintf(db.SPPROXYSRV, "%v: PastLastMount %v %v", sca.sc.ClntId(), req, rep)
	return nil
}

func (sca *SPProxySrvAPI) NewRootMount(ctx fs.CtxI, req scproto.SigmaMountTreeReq, rep *scproto.SigmaErrRep) error {
	err := sca.sc.NewRootMount(req.Tree, req.MountName)
	rep.Err = sca.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: NewRootMount %v %v", sca.sc.ClntId(), req, rep)
	return nil
}

func (sca *SPProxySrvAPI) Mounts(ctx fs.CtxI, req scproto.SigmaNullReq, rep *scproto.SigmaMountsRep) error {
	mnts := sca.sc.Mounts()
	rep.Endpoints = mnts
	rep.Err = sca.setErr(nil)
	db.DPrintf(db.SPPROXYSRV, "%v: Mounts %v %v", sca.sc.ClntId(), req, rep)
	return nil
}

func (sca *SPProxySrvAPI) Detach(ctx fs.CtxI, req scproto.SigmaPathReq, rep *scproto.SigmaErrRep) error {
	err := sca.sc.Detach(req.Path)
	rep.Err = sca.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: Detach %v %v", sca.sc.ClntId(), req, rep)
	return nil
}

func (sca *SPProxySrvAPI) Disconnect(ctx fs.CtxI, req scproto.SigmaPathReq, rep *scproto.SigmaErrRep) error {
	err := sca.sc.Disconnect(req.Path)
	rep.Err = sca.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: Disconnect %v %v", sca.sc.ClntId(), req, rep)
	return nil
}

func (sca *SPProxySrvAPI) Close(ctx fs.CtxI, req scproto.SigmaNullReq, rep *scproto.SigmaErrRep) error {
	db.DPrintf(db.ALWAYS, "%v: Close fslib %v", sca.sc.ClntId(), sca)
	var err error
	if !sca.testAndSetClosed() {
		err = sca.sc.Close()
	} else {
		err = nil
	}
	rep.Err = sca.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: Close %v %v", sca.sc.ClntId(), req, rep)
	return nil
}

// ========== EP Manipulation =========
func (sca *SPProxySrvAPI) RegisterEP(ctx fs.CtxI, req scproto.SigmaRegisterEPReq, rep *scproto.SigmaErrRep) error {
	ep := sp.NewEndpointFromProto(req.Endpoint)
	useEPCC := sca.epcc != nil
	db.DPrintf(db.SPPROXYSRV, "%v: RegisterEP (useEPCC:%v) %v -> %v", sca.sc.ClntId(), useEPCC, req.Path, ep)
	var err error
	if !useEPCC {
		if err = sca.sc.MkEndpointFile(req.Path, ep); err != nil {
			db.DPrintf(db.SPPROXYSRV_ERR, "%v: RegisterEP MkEndpointFile err: %v", sca.sc.ClntId(), err)
		}
	} else {
		if err = sca.epcc.RegisterEndpoint(req.Path, sca.sc.ProcEnv().GetPID().String(), ep); err != nil {
			db.DPrintf(db.SPPROXYSRV_ERR, "%v: RegisterEP EPCC err: %v", sca.sc.ClntId(), err)
		}
	}
	rep.Err = sca.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: RegisterEP (useEPCC:%v) done %v %v", sca.sc.ClntId(), useEPCC, req, rep)
	return nil
}

// ========== Procclnt API ==========
func (sca *SPProxySrvAPI) Started(ctx fs.CtxI, req scproto.SigmaNullReq, rep *scproto.SigmaErrRep) error {
	db.DPrintf(db.SPPROXYSRV, "%v: Started", sca.sc.ClntId())
	start := time.Now()
	err := sca.sc.Started()
	if err != nil {
		db.DPrintf(db.SPPROXYSRV_ERR, "%v: Started err: %v", sca.sc.ClntId(), err)
	}
	perf.LogSpawnLatency("SPProxySrv.Started.Started", sca.sc.ProcEnv().GetPID(), sca.sc.ProcEnv().GetSpawnTime(), start)
	rep.Err = sca.setErr(err)
	db.DPrintf(db.SPPROXYSRV, "%v: Started done %v %v", sca.sc.ClntId(), req, rep)
	return nil
}

func (sca *SPProxySrvAPI) Exited(ctx fs.CtxI, req scproto.SigmaExitedReq, rep *scproto.SigmaErrRep) error {
	status := proc.Tstatus(req.Status)
	db.DPrintf(db.SPPROXYSRV, "%v: Exited status %v  msg %v", sca.sc.ClntId(), status, req.Msg)
	sca.sc.Exited(proc.NewStatusInfo(proc.Tstatus(req.Status), req.Msg, nil))
	db.DPrintf(db.SPPROXYSRV, "%v: Exited done %v %v", sca.sc.ClntId(), req, rep)
	return nil
}

func (sca *SPProxySrvAPI) WaitEvict(ctx fs.CtxI, req scproto.SigmaNullReq, rep *scproto.SigmaErrRep) error {
	db.DPrintf(db.SPPROXYSRV, "%v: WaitEvict %v", sca.sc.ClntId())
	sca.sc.WaitEvict(sca.sc.ProcEnv().GetPID())
	db.DPrintf(db.SPPROXYSRV, "%v: WaitEvict done %v %v", sca.sc.ClntId(), req, rep)
	return nil
}

// ========== Delegated RPCs ==========
func (sca *SPProxySrvAPI) GetDelegatedRPCReply(ctx fs.CtxI, req scproto.SigmaDelegatedRPCReq, rep *scproto.SigmaDelegatedRPCRep) error {
	db.DPrintf(db.SPPROXYSRV, "%v: GetDelegatedRPCReply %v", sca.sc.ClntId(), req)
	iov, err := sca.spps.psm.GetReply(sca.sc.ProcEnv().GetPID(), req.RPCIdx)
	rep.Blob = &rpcproto.Blob{
		Iov: [][]byte(iov),
	}
	rep.Err = sca.setErr(err)
	lens := make([]int, len(iov)+1)
	if db.WillBePrinted(db.SPPROXYSRV) {
		lens[0] = len(iov)
		for i := range iov {
			lens[i+1] = len(iov[i])
		}
	}
	db.DPrintf(db.SPPROXYSRV, "%v: GetDelegatedRPCReply done %v lens %v", sca.sc.ClntId(), req, lens)
	return nil
}

func (sca *SPProxySrvAPI) GetMultiDelegatedRPCReplies(ctx fs.CtxI, req scproto.SigmaMultiDelegatedRPCReq, rep *scproto.SigmaMultiDelegatedRPCRep) error {
	db.DPrintf(db.SPPROXYSRV, "%v: GetMultiDelegatedRPCReply %v", sca.sc.ClntId(), req)
	iovs := make([]sessp.IoVec, 0, len(req.RPCIdxs))
	rep.Blob = &rpcproto.Blob{
		Iov: make(sessp.IoVec, 0),
	}
	for _, rpcIdx := range req.RPCIdxs {
		iov, err := sca.spps.psm.GetReply(sca.sc.ProcEnv().GetPID(), rpcIdx)
		iovs = append(iovs, iov)
		rep.Blob.Iov = append(rep.Blob.Iov, iov...)
		rep.NIOVs = append(rep.NIOVs, uint64(len(iov)))
		rep.Errs = append(rep.Errs, sca.setErr(err))
	}
	//	lens := make([]int, len(iov)+1)
	//	if db.WillBePrinted(db.SPPROXYSRV) {
	//		lens[0] = len(iov)
	//		for i := range iov {
	//			lens[i+1] = len(iov[i])
	//		}
	//	}
	db.DPrintf(db.SPPROXYSRV, "%v: GetMultiDelegatedRPCReply done %v", sca.sc.ClntId(), req)
	return nil
}
