// The CtrlClnt allows procd to control its local spproxyd
package clnt

import (
	db "sigmaos/debug"
	"sigmaos/proc"
	spproto "sigmaos/proxy/sigmap/proto"
	wasmrpc "sigmaos/proxy/wasm/rpc"
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

// Wait until a proc's cosandbox has completed
func (scc *CtrlClnt) WaitCoSandboxCompletion(pid sp.Tpid) (wasmrpc.Tstatus, string, error) {
	db.DPrintf(db.SPPROXYCLNT, "[%v] Wait for boot script completion", pid)
	req := spproto.SigmaWaitCoSandboxReq{
		PIDStr: pid.String(),
	}
	rep := spproto.SigmaWaitCoSandboxRep{}
	err := scc.rpcc.RPC("CtrlAPI.WaitCoSandboxCompletion", &req, &rep)
	status := wasmrpc.Tstatus(rep.Status)
	msg := rep.Msg
	db.DPrintf(db.SPPROXYCLNT, "[%v] Done waiting for boot script completion: %v %v", pid, status, msg)
	if err != nil {
		return 0, sp.NOT_SET, err
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return 0, sp.NOT_SET, sp.NewErr(rep.Err)
	}
	return status, msg, nil
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

// Get the TCP port spproxyd is listening on (only valid when blink is enabled)
func (scc *CtrlClnt) GetTCPPort() (int, error) {
	req := spproto.SigmaNullReq{}
	rep := spproto.SigmaTCPPortRep{}
	err := scc.rpcc.RPC("CtrlAPI.GetTCPPort", &req, &rep)
	if err != nil {
		return 0, err
	}
	if rep.Err.TErrCode() != serr.TErrNoError {
		return 0, sp.NewErr(rep.Err)
	}
	return int(rep.Port), nil
}

// Close the socket connection, which closes dmxclnt too.
func (scc *CtrlClnt) close() error {
	return scc.rpcc.Channel().(*rpcchan.RPCChannel).Conn().Close()
}
