// Package spproxyclnt is an RPC-based client for proxing the
// [sigmaos] interface. It sends request to [spproxysrv], which has a
// full sigmaclnt to interact with SigmaOS.
package clnt

import (
	"net"

	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/proc"
	spproto "sigmaos/proxy/sigmap/proto"
	"sigmaos/rpc"
	rpcclnt "sigmaos/rpc/clnt"
	"sigmaos/rpc/clnt/channel/rpcchannel"
	rpcclntopts "sigmaos/rpc/clnt/opts"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
	"sigmaos/util/io/demux"
)

type SPProxyClnt struct {
	pe           *proc.ProcEnv
	dmx          *demux.DemuxClnt
	rpcc         *rpcclnt.RPCClnt
	npc          *dialproxyclnt.DialProxyClnt
	seqcntr      *sessp.Tseqcntr
	conn         net.Conn
	disconnected bool
}

func NewSPProxyClnt(pe *proc.ProcEnv, npc *dialproxyclnt.DialProxyClnt) (*SPProxyClnt, error) {
	conn, err := net.Dial("unix", sp.SIGMASOCKET)
	if err != nil {
		return nil, err
	}
	scc := &SPProxyClnt{
		pe:           pe,
		npc:          npc,
		dmx:          nil,
		rpcc:         nil,
		seqcntr:      new(sessp.Tseqcntr),
		conn:         conn,
		disconnected: false,
	}

	ch := rpcchannel.NewRPCChannel(conn)
	rpcc, err := rpcclnt.NewRPCClnt("spproxy", rpcclntopts.WithRPCChannel(ch))
	if err != nil {
		return nil, err
	}
	scc.rpcc = rpcc
	// Initialize the server-side component of sigmaclnt by sending the proc env
	db.DPrintf(db.SPPROXYCLNT, "Init sigmaclntclnt for %v", pe.GetPID())
	if err := scc.Init(); err != nil {
		db.DPrintf(db.ERROR, "Error init sigmaclnt: %v", err)
		return nil, err
	}
	return scc, nil
}

func (scc *SPProxyClnt) StatsSrv() (*rpc.RPCStatsSnapshot, error) {
	return nil, nil
}

// Tell spproxyd to shut down
func (scc *SPProxyClnt) Shutdown() error {
	req := spproto.SigmaNullReq{}
	rep := spproto.SigmaErrRep{}
	err := scc.rpcErr("SPProxySrvAPI.Shutdown", &req, &rep)
	db.DPrintf(db.SPPROXYCLNT, "Shutdown %v %v %v", req, rep, err)
	return err
}

// Close the socket connection, which closes dmxclnt too.
func (scc *SPProxyClnt) close() error {
	return scc.conn.Close()
}
