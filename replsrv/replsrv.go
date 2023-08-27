package replsrv

import (
	"sync"

	replproto "sigmaos/repl/proto"

	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/proc"
	"sigmaos/repl"
	"sigmaos/replraft"
	"sigmaos/rpcsrv"
)

//
// Replicated CacheSrv with same RPC interface (CacheSrv) has unreplicated CacheSrv
//

type ReplSrv struct {
	mu      sync.Mutex
	raftcfg *replraft.RaftConfig
	replSrv repl.Server
	rpcs    *rpcsrv.RPCSrv
	rt      *ReplyTable
}

func NewReplSrv(raftcfg *replraft.RaftConfig, svci any) *ReplSrv {
	rs := &ReplSrv{
		raftcfg: raftcfg,
		rpcs:    rpcsrv.NewRPCSrv(svci, nil),
		rt:      NewReplyTable(),
	}
	rs.replSrv = raftcfg.MakeServer(rs.applyOp)
	rs.replSrv.Start()
	db.DPrintf(db.ALWAYS, "Starting repl server: %v %v", svci, raftcfg)
	return rs
}

func (rs *ReplSrv) applyOp(req *replproto.ReplOpRequest, rep *replproto.ReplOpReply) error {
	db.DPrintf(db.REPLSRV, "ApplyOp %v\n", req)
	duplicate, err, b := rs.rt.IsDuplicate(req.TclntId(), req.Tseqno())
	if duplicate {
		db.DPrintf(db.REPLSRV, "ApplyOp duplicate %v\n", req)
		if rep != nil {
			rep.Msg = b
		}
		return err
	}
	if b, err := rs.rpcs.ServeRPC(ctx.MkCtxNull(), req.Method, req.Msg); err != nil {
		rs.rt.PutReply(req.TclntId(), req.Tseqno(), err, nil)
		return err
	} else {
		if rep != nil {
			rep.Msg = b
		}
		rs.rt.PutReply(req.TclntId(), req.Tseqno(), nil, b)
	}
	return nil
}

func (rs *ReplSrv) ProcessOp(ctx fs.CtxI, req replproto.ReplOpRequest, rep *replproto.ReplOpReply) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	db.DPrintf(db.REPLSRV, "ProcessOp: submit %v\n", req)
	if err := rs.replSrv.Process(&req, rep); err != nil {
		db.DPrintf(db.REPLSRV, "ProcessOp: op done %v err %v\n", req, err)
		return err
	}
	db.DPrintf(db.REPLSRV, "ProcessOp: op done %v rep %v\n", req, rep)
	return nil
}
