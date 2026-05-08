// Package spproxyclnt is an RPC-based client for proxing the
// [sigmaos] interface. It sends request to [spproxysrv], which has a
// full sigmaclnt to interact with SigmaOS.
package clnt

import (
	sos "sigmaos/api/sigmaos"
	db "sigmaos/debug"
	dialproxyclnt "sigmaos/dialproxy/clnt"
	"sigmaos/proc"
	spproto "sigmaos/proxy/sigmap/proto"
	"sigmaos/rpc"
	rpcclnt "sigmaos/rpc/clnt"
	"sigmaos/rpc/clnt/channel"
	rpcchan "sigmaos/rpc/clnt/channel/rpcchannel"
	rpcnc "sigmaos/rpc/clnt/netconn"
	"sigmaos/serr"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
	"sigmaos/shmem"
	"sigmaos/util/io/demux"
)

type SPProxyClnt struct {
	pe           *proc.ProcEnv
	dmx          *demux.DemuxClnt
	rpcc         *rpcclnt.RPCClnt
	npc          *dialproxyclnt.DialProxyClnt
	seqcntr      *sessp.Tseqcntr
	disconnected bool
	useShmem     bool
	shm          *shmem.Segment
}

func NewSPProxyClnt(pe *proc.ProcEnv, npc *dialproxyclnt.DialProxyClnt) (*SPProxyClnt, error) {
	rpcc, err := rpcnc.NewUnixRPCClnt("spproxy", sp.SIGMASOCKET)
	if err != nil {
		return nil, err
	}
	scc := &SPProxyClnt{
		pe:           pe,
		npc:          npc,
		dmx:          nil,
		rpcc:         rpcc,
		seqcntr:      new(sessp.Tseqcntr),
		disconnected: false,
	}
	// Initialize the server-side component of sigmaclnt by sending the proc env
	db.DPrintf(db.SPPROXYCLNT, "Init sigmaproxyclnt for %v", pe.GetPID())
	if err := scc.Init(); err != nil {
		db.DPrintf(db.ERROR, "Error init sigmaclnt: %v", err)
		return nil, err
	}
	// Open the shmem segment created by the server during proc spawn.
	if pe.GetUseShmem() {
		scc.shm, err = shmem.NewSegment(pe.GetPID().String(), pe.GetShmemMB()*proc.Tmem(sp.MBYTE), false)
		if err != nil {
			return nil, err
		}
	}
	return scc, nil
}

func (scc *SPProxyClnt) SetUseShmem(v bool) {
	scc.useShmem = v
}

func (scc *SPProxyClnt) GetShmemSegment() *shmem.Segment {
	return scc.shm
}

func (scc *SPProxyClnt) GetRPCChannel() channel.RPCChannel {
	return scc.rpcc.Channel()
}

func (scc *SPProxyClnt) Stats() (*sos.ClntStats, error) {
	return nil, serr.NewErr(serr.TErrNotSupported, "Stats")
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
	return scc.rpcc.Channel().(*rpcchan.RPCChannel).Conn().Close()
}
