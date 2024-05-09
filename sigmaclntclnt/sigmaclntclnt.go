// Package sigmaclntclnt is an RPC-based client for proxing the
// [sigmaos] interface. It sends request to [sigmclntsrv], which has a
// full sigmaclnt to interact with SigmaOS.
package sigmaclntclnt

import (
	"fmt"
	"net"

	db "sigmaos/debug"
	"sigmaos/demux"
	"sigmaos/netproxyclnt"
	"sigmaos/proc"
	"sigmaos/rpc"
	"sigmaos/rpcclnt"
	"sigmaos/sessp"
	"sigmaos/sigmaclntcodec"
	scproto "sigmaos/sigmaclntsrv/proto"
	sp "sigmaos/sigmap"
)

type SigmaClntClnt struct {
	pe           *proc.ProcEnv
	dmx          *demux.DemuxClnt
	rpcc         *rpcclnt.RPCClnt
	npc          *netproxyclnt.NetProxyClnt
	seqcntr      *sessp.Tseqcntr
	conn         net.Conn
	disconnected bool
}

func NewSigmaClntClnt(pe *proc.ProcEnv, npc *netproxyclnt.NetProxyClnt) (*SigmaClntClnt, error) {
	conn, err := net.Dial("unix", sp.SIGMASOCKET)
	if err != nil {
		return nil, err
	}
	scc := &SigmaClntClnt{
		pe:           pe,
		npc:          npc,
		dmx:          nil,
		rpcc:         nil,
		seqcntr:      new(sessp.Tseqcntr),
		conn:         conn,
		disconnected: false,
	}

	iovm := demux.NewIoVecMap()
	scc.dmx = demux.NewDemuxClnt(sigmaclntcodec.NewTransport(conn, iovm), iovm)
	scc.rpcc = rpcclnt.NewRPCClnt(scc)
	// Initialize the server-side component of sigmaclnt by sending the proc env
	db.DPrintf(db.SIGMACLNTCLNT, "Init sigmaclntclnt for %v", pe.GetPID())
	if err := scc.Init(); err != nil {
		db.DPrintf(db.ERROR, "Error init sigmaclnt: %v", err)
		return nil, err
	}
	return scc, nil
}

func (scc *SigmaClntClnt) SendReceive(iniov sessp.IoVec, outiov sessp.IoVec) error {
	c := sigmaclntcodec.NewCall(sessp.NextSeqno(scc.seqcntr), iniov)
	rep, err := scc.dmx.SendReceive(c, outiov)
	if err != nil {
		return err
	} else {
		c := rep.(*sigmaclntcodec.Call)
		if len(outiov) != len(c.Iov) {
			return fmt.Errorf("sigmaclntclnt outiov len wrong: %v != %v", len(outiov), len(c.Iov))
		}
		return nil
	}
}

func (scc *SigmaClntClnt) StatsSrv() (*rpc.RPCStatsSnapshot, error) {
	return nil, nil
}

func (scc *SigmaClntClnt) ReportError(err error) {
	db.DPrintf(db.DEMUXCLNT, "ReportError %v", err)
	go func() {
		scc.close()
	}()
}

// Tell sigmaclntd to shut down
func (scc *SigmaClntClnt) Shutdown() error {
	req := scproto.SigmaNullRequest{}
	rep := scproto.SigmaErrReply{}
	err := scc.rpcErr("SigmaClntSrvAPI.Shutdown", &req, &rep)
	db.DPrintf(db.SIGMACLNTCLNT, "Shutdown %v %v %v", req, rep, err)
	return err
}

// Close the socket connection, which closes dmxclnt too.
func (scc *SigmaClntClnt) close() error {
	return scc.conn.Close()
}
