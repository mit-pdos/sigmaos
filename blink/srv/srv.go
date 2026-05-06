package srv

import (
	"fmt"
	"net"

	"sigmaos/api/fs"
	blinkproto "sigmaos/blink/proto"
	"sigmaos/blink"
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/proc"
	rpcsrv "sigmaos/rpc/srv"
	"sigmaos/rpc/transport"
	"sigmaos/serr"
	"sigmaos/util/io/demux"
)

type BlinkSrv struct {
	kernelId string
}

func newBlinkSrv(kernelId string) *BlinkSrv {
	return &BlinkSrv{kernelId: kernelId}
}

func (bs *BlinkSrv) runServer() error {
	addr := fmt.Sprintf(":%d", blink.BLINK_PORT)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	db.DPrintf(db.BLINKD, "BlinkSrv listening on %v", addr)
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		newBlinkSrvConn(conn, bs)
	}
}

type BlinkSrvConn struct {
	conn net.Conn
	ctx  fs.CtxI
	dmx  *demux.DemuxSrv
	rpcs *rpcsrv.RPCSrv
}

func newBlinkSrvConn(conn net.Conn, bs *BlinkSrv) *BlinkSrvConn {
	api := &BlinkSrvAPI{bs: bs}
	bsc := &BlinkSrvConn{
		conn: conn,
		ctx:  ctx.NewCtxNull(),
	}
	iovm := demux.NewIoVecMap()
	bsc.rpcs = rpcsrv.NewRPCSrv(api, nil)
	bsc.dmx = demux.NewDemuxSrv(bsc, transport.NewTransport(conn, iovm))
	return bsc
}

func (bsc *BlinkSrvConn) ServeRequest(c demux.CallI) (demux.CallI, *serr.Err) {
	req := c.(*transport.Call)
	rep, err := bsc.rpcs.WriteRead(bsc.ctx, req.Iov)
	if err != nil {
		db.DPrintf(db.BLINKD, "BlinkSrvConn ServeRequest err %v", err)
	}
	return transport.NewCall(req.Seqno, rep), nil
}

func (bsc *BlinkSrvConn) ReportError(err error) {
	db.DPrintf(db.BLINKD, "BlinkSrvConn ReportError err %v", err)
	go bsc.conn.Close()
}

type BlinkSrvAPI struct {
	bs *BlinkSrv
}

func (api *BlinkSrvAPI) RunBlinkProc(ctx fs.CtxI, req blinkproto.RunBlinkProcReq, rep *blinkproto.RunBlinkProcRep) error {
	p := proc.NewProcFromProto(req.Proc)
	db.DPrintf(db.BLINKD, "BlinkSrvAPI.RunBlinkProc %v kid %v", p, req.Kid)
	// TODO: invoke junction_run to restore and run the proc
	return nil
}

func RunBlinkSrv(kernelId string) error {
	bs := newBlinkSrv(kernelId)
	return bs.runServer()
}
