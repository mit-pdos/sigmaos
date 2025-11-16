// The CtrlClnt allows procd to control its local spproxyd
package clnt

import (
	db "sigmaos/debug"
	"sigmaos/proc"
	spproto "sigmaos/proxy/sigmap/proto"
	"sigmaos/rpc"
	rpcclnt "sigmaos/rpc/clnt"
	rpcchan "sigmaos/rpc/clnt/channel/rpcchannel"
	rpcnc "sigmaos/rpc/clnt/netconn"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
	"sigmaos/util/io/demux"
)

type CtrlClnt struct {
	dmx          *demux.DemuxClnt
	rpcc         *rpcclnt.RPCClnt
	disconnected bool
}

func NewCtrlClnt() (*CtrlClnt, error) {
	rpcc, err := rpcnc.NewUnixRPCClnt("spproxyctrl", sp.SIGMASOCKET_CTRL)
	if err != nil {
		return nil, err
	}
	return &CtrlClnt{
		dmx:          nil,
		rpcc:         rpcc,
		disconnected: false,
	}, nil
}

func (scc *CtrlClnt) StatsSrv() (*rpc.RPCStatsSnapshot, error) {
	return nil, nil
}

// Tell spproxyd to prepare for an incoming proc
func (scc *CtrlClnt) InformIncomingProc(p *proc.Proc) error {
	// Create a copy of the proc proto
	pp := *p.GetProto()
	// Clear the env, which is unneeded by spproxyd and may be modified in
	// another thread
	pp.Env = nil
	req := spproto.SigmaInformProcReq{
		ProcProto: &pp,
	}
	rep := spproto.SigmaErrRep{}
	err := scc.rpcc.RPC("CtrlAPI.InformIncomingProc", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "Inform incoming proc %v", p)
	if err != nil {
		return err
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return sp.NewErr(rep.Err)
	}
	return nil
}

// Wait until a proc's bootscript has completed
func (scc *CtrlClnt) WaitBootScriptCompletion(pid sp.Tpid) error {
	db.DPrintf(db.SPPROXYCLNT, "[%v] Wait for boot script completion", pid)
	req := spproto.SigmaWaitBootScriptReq{
		PIDStr: pid.String(),
	}
	rep := spproto.SigmaErrRep{}
	err := scc.rpcc.RPC("CtrlAPI.WaitBootScriptCompletion", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "[%v] Done waiting for boot script completion", pid)
	if err != nil {
		return err
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return sp.NewErr(rep.Err)
	}
	return nil
}

// Tell spproxyd a proc is done
func (scc *CtrlClnt) InformProcDone(p *proc.Proc) error {
	req := spproto.SigmaInformProcReq{
		ProcProto: p.GetProto(),
	}
	rep := spproto.SigmaErrRep{}
	err := scc.rpcc.RPC("CtrlAPI.InformProcDone", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "Inform proc done %v", p)
	if err != nil {
		return err
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return sp.NewErr(rep.Err)
	}
	return nil
}

// Close the socket connection, which closes dmxclnt too.
func (scc *CtrlClnt) close() error {
	return scc.rpcc.Channel().(*rpcchan.RPCChannel).Conn().Close()
}
