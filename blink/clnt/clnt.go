package clnt

import (
	"sigmaos/blink"
	blinkproto "sigmaos/blink/proto"
	db "sigmaos/debug"
	"sigmaos/proc"
	rpcclnt "sigmaos/rpc/clnt"
	rpcnc "sigmaos/rpc/clnt/netconn"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type BlinkClnt struct {
	rpcc *rpcclnt.RPCClnt
}

func NewBlinkClnt() (*BlinkClnt, error) {
	ep := sp.NewEndpoint(sp.EXTERNAL_EP, sp.Taddrs{sp.NewTaddr("127.0.0.1", sp.Tport(blink.BLINK_PORT))})
	rpcc, err := rpcnc.NewTCPRPCClnt("blinkd", ep, 0)
	if err != nil {
		db.DPrintf(db.BLINKD, "BlinkClnt dial err: %v", err)
		return nil, err
	}
	return &BlinkClnt{rpcc: rpcc}, nil
}

func (bc *BlinkClnt) RunBlinkProc(p *proc.Proc, kid string, procSrvPID string, spproxyTCPHost string, spproxyTCPPort int) error {
	req := &blinkproto.RunBlinkProcReq{
		Proc:            p.GetProto(),
		Kid:             kid,
		ProcsrvPid:      procSrvPID,
		SpproxyTcpHost:  spproxyTCPHost,
		SpproxyTcpPort:  uint32(spproxyTCPPort),
	}
	rep := &blinkproto.RunBlinkProcRep{}
	if err := bc.rpcc.RPC("BlinkSrvAPI.RunBlinkProc", req, rep); err != nil {
		db.DPrintf(db.BLINKD, "BlinkClnt.RunBlinkProc RPC err: %v", err)
		return err
	}
	db.DPrintf(db.BLINKD, "BlinkClnt.RunBlinkProc done err=%v", rep.Err)
	if rep.Err.TErrCode() != serr.TErrNoError {
		return sp.NewErr(rep.Err)
	}
	return nil
}
