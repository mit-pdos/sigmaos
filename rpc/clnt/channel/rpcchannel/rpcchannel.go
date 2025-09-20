package rpcchannel

import (
	"fmt"
	"net"

	db "sigmaos/debug"
	"sigmaos/rpc"
	"sigmaos/rpc/transport"
	sessp "sigmaos/session/proto"
	"sigmaos/util/io/demux"
)

// A generic RPC channel over a network connection
type RPCChannel struct {
	dmx     *demux.DemuxClnt
	seqcntr *sessp.Tseqcntr
	conn    net.Conn
}

func NewRPCChannel(conn net.Conn) *RPCChannel {
	iovm := demux.NewIoVecMap()
	ch := &RPCChannel{
		dmx:     demux.NewDemuxClnt(transport.NewTransport(conn, iovm), iovm),
		seqcntr: new(sessp.Tseqcntr),
		conn:    conn,
	}
	return ch
}

func (ch *RPCChannel) SendReceive(iniov *sessp.IoVec, outiov *sessp.IoVec) error {
	c := transport.NewCall(sessp.NextSeqno(ch.seqcntr), iniov)
	rep, err := ch.dmx.SendReceive(c, outiov)
	if err != nil {
		return err
	} else {
		c := rep.(*transport.Call)
		if outiov.Len() != c.Iov.Len() {
			return fmt.Errorf("outiov len wrong: %v != %v", outiov.Len(), c.Iov.Len())
		}
		return nil
	}
}

func (ch *RPCChannel) StatsSrv() (*rpc.RPCStatsSnapshot, error) {
	return nil, nil
}

func (ch *RPCChannel) ReportError(err error) {
	db.DPrintf(db.RPCCHAN, "ReportError %v", err)
	go func() {
		ch.close()
	}()
}

// Close the socket connection, which closes dmxclnt too.
func (ch *RPCChannel) close() error {
	return ch.conn.Close()
}

func (ch *RPCChannel) Conn() net.Conn {
	return ch.conn
}
