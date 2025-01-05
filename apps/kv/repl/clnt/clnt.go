package clnt

import (
	"sync/atomic"

	"google.golang.org/protobuf/proto"

	replproto "sigmaos/apps/kv/repl/proto"
	rpcclnt "sigmaos/rpc/clnt"
	sprpcclnt "sigmaos/rpc/clnt/sigmap"
	"sigmaos/sigmaclnt/fslib"
	sp "sigmaos/sigmap"
)

type ReplClnt struct {
	*rpcclnt.ClntCache
	cid   sp.TclntId
	seqno atomic.Uint64
}

func NewReplClnt(fsl *fslib.FsLib) *ReplClnt {
	rc := &ReplClnt{
		cid:       fsl.ClntId(),
		ClntCache: rpcclnt.NewRPCClntCache(sprpcclnt.WithSPChannel(fsl)),
	}
	return rc
}

func (rc *ReplClnt) nextSeqno() sp.Tseqno {
	return sp.Tseqno(rc.seqno.Add(1))
}

func (rc *ReplClnt) NewReplOp(method, key string, val proto.Message) (*replproto.ReplOpReq, error) {
	b, err := proto.Marshal(val)
	if err != nil {
		return nil, err
	}
	seqno := rc.nextSeqno()
	return &replproto.ReplOpReq{
		Method: method,
		ClntId: uint32(rc.cid),
		Seqno:  uint64(seqno),
		Msg:    b,
	}, nil
}

func (rc *ReplClnt) ReplOp(srv, method, key string, val proto.Message) ([]byte, error) {
	req, err := rc.NewReplOp(method, key, val)
	if err != nil {
		return nil, err
	}
	var res replproto.ReplOpRep
	if err := rc.RPC(srv, "ReplSrv.ProcessOp", req, &res); err != nil {
		return nil, err
	}
	return res.Msg, nil
}
