package srv

import (
	"net"

	"sigmaos/api/fs"
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/proc"
	scproto "sigmaos/proxy/sigmap/proto"
	rpcsrv "sigmaos/rpc/srv"
	"sigmaos/rpc/transport"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/util/io/demux"
)

type CtrlAPI struct {
	cc *CtrlConn
}

type CtrlConn struct {
	conn net.Conn
	ctx  fs.CtxI
	dmx  *demux.DemuxSrv
	rpcs *rpcsrv.RPCSrv
	spps *SPProxySrv
}

func newCtrlConn(conn net.Conn, spps *SPProxySrv) *CtrlConn {
	db.DPrintf(db.SPPROXYSRV, "Create SigmaCtrlAPI")
	cc := &CtrlConn{
		conn: conn,
		ctx:  ctx.NewCtxNull(),
		spps: spps,
	}
	iovm := demux.NewIoVecMap()
	capi := &CtrlAPI{
		cc: cc,
	}
	cc.rpcs = rpcsrv.NewRPCSrv(capi, nil)
	cc.dmx = demux.NewDemuxSrv(cc, transport.NewTransport(conn, iovm))
	return cc
}

func (capi *CtrlAPI) InformIncomingProc(ctx fs.CtxI, req scproto.SigmaInformProcReq, rep *scproto.SigmaErrRep) error {
	p := proc.NewProcFromProto(req.ProcProto)
	capi.cc.spps.IncomingProc(p)
	rep.Err = sp.NewRerror()
	return nil
}

func (capi *CtrlAPI) WaitCoSandboxCompletion(ctx fs.CtxI, req scproto.SigmaWaitCoSandboxReq, rep *scproto.SigmaWaitCoSandboxRep) error {
	status, msg, err := capi.cc.spps.WaitCoSandboxCompletion(sp.Tpid(req.PIDStr))
	if err != nil {
		rep.Err = sp.NewRerrorErr(err)
	} else {
		rep.Err = sp.NewRerror()
	}
	rep.Status = uint64(status)
	rep.Msg = msg
	return nil
}

func (capi *CtrlAPI) InformProcDone(ctx fs.CtxI, req scproto.SigmaInformProcReq, rep *scproto.SigmaErrRep) error {
	p := proc.NewProcFromProto(req.ProcProto)
	capi.cc.spps.ProcDone(p)
	rep.Err = sp.NewRerror()
	return nil
}

func (capi *CtrlAPI) GetTCPPort(ctx fs.CtxI, req scproto.SigmaNullReq, rep *scproto.SigmaTCPPortRep) error {
	rep.Port = uint32(capi.cc.spps.tcpPort)
	rep.Err = sp.NewRerror()
	return nil
}

func (cc *CtrlConn) close() {
	db.DPrintf(db.SPPROXYSRV, "close ctrl conn")
}

func (cc *CtrlConn) ServeRequest(c demux.CallI) (demux.CallI, *serr.Err) {
	req := c.(*transport.Call)
	rep, err := cc.rpcs.WriteRead(cc.ctx, req.Iov)
	if err != nil {
		db.DPrintf(db.SPPROXYSRV, "ServeRequest: writeRead err %v", err)
	}
	return transport.NewCall(req.Seqno, rep), nil
}

func (cc *CtrlConn) ReportError(err error) {
	db.DPrintf(db.SPPROXYSRV, "CtrlAPI ReportError err %v", err)
	go func() {
		cc.close()
	}()
}
