// Package replsrv replicates a svci using Raft.  Each replica
// maintains a reply table to filter out duplicate requests.
package srv

import (
	"sync"

	replproto "sigmaos/apps/kv/repl/proto"

	"sigmaos/api/fs"
	"sigmaos/apps/kv/repl"
	"sigmaos/apps/kv/repl/raft"
	"sigmaos/ctx"
	db "sigmaos/debug"
	rpcsrv "sigmaos/rpc/srv"
	sessp "sigmaos/session/proto"
)

type ReplSrv struct {
	mu      sync.Mutex
	raftcfg *raft.RaftConfig
	replSrv repl.Server
	rpcs    *rpcsrv.RPCSrv
	rt      *ReplyTable
}

func NewReplSrv(raftcfg *raft.RaftConfig, svci any) (*ReplSrv, error) {
	var err error
	rs := &ReplSrv{
		raftcfg: raftcfg,
		rpcs:    rpcsrv.NewRPCSrv(svci, nil),
		rt:      NewReplyTable(),
	}
	rs.replSrv, err = raftcfg.NewServer(rs.applyOp)
	if err != nil {
		return nil, err
	}
	rs.replSrv.Start()
	db.DPrintf(db.ALWAYS, "Starting repl server: %v %v", svci, raftcfg)
	return rs, nil
}

func (rs *ReplSrv) applyOp(req *replproto.ReplOpReq, rep *replproto.ReplOpRep) error {
	db.DPrintf(db.REPLSRV, "ApplyOp %v\n", req)
	duplicate, err, b := rs.rt.IsDuplicate(req.TclntId(), req.Tseqno())
	if duplicate {
		db.DPrintf(db.REPLSRV, "ApplyOp duplicate %v\n", req)
		if rep != nil {
			rep.Msg = b
		}
		return err
	}
	iov := sessp.IoVec{req.Msg}
	if iov, err := rs.rpcs.ServeRPC(ctx.NewCtxNull(), req.Method, iov); err != nil {
		rs.rt.PutReply(req.TclntId(), req.Tseqno(), err, nil)
		return err
	} else {
		if rep != nil {
			rep.Msg = iov[0]
		}
		rs.rt.PutReply(req.TclntId(), req.Tseqno(), nil, iov[0])
	}
	return nil
}

func (rs *ReplSrv) ProcessOp(ctx fs.CtxI, req replproto.ReplOpReq, rep *replproto.ReplOpRep) error {
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
