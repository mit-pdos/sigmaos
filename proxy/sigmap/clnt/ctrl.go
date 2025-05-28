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
	req := spproto.SigmaInformProcReq{
		ProcEnvProto: p.GetProcEnv().GetProto(),
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

// Tell spproxyd a proc is done
func (scc *CtrlClnt) InformProcDone(p *proc.Proc) error {
	req := spproto.SigmaInformProcReq{
		ProcEnvProto: p.GetProcEnv().GetProto(),
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
