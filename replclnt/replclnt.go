package replclnt

import (
	"sync/atomic"

	"google.golang.org/protobuf/proto"

	"sigmaos/fslib"
	replproto "sigmaos/repl/proto"
	rpcclnt "sigmaos/rpc/clnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmarpcchan"
)

type ReplClnt struct {
	*rpcclnt.ClntCache
	cid   sp.TclntId
	seqno atomic.Uint64
}

func NewReplClnt(fsls []*fslib.FsLib) *ReplClnt {
	rc := &ReplClnt{
		cid:       fsls[0].ClntId(),
		ClntCache: rpcclnt.NewRPCClntCache(sigmarpcchan.SigmaRPCChanFactory(fsls)),
	}
	return rc
}

func (rc *ReplClnt) nextSeqno() sp.Tseqno {
	return sp.Tseqno(rc.seqno.Add(1))
}

func (rc *ReplClnt) NewReplOp(method, key string, val proto.Message) (*replproto.ReplOpRequest, error) {
	b, err := proto.Marshal(val)
	if err != nil {
		return nil, err
	}
	seqno := rc.nextSeqno()
	return &replproto.ReplOpRequest{
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
	var res replproto.ReplOpReply
	if err := rc.RPC(srv, "ReplSrv.ProcessOp", req, &res); err != nil {
		return nil, err
	}
	return res.Msg, nil
}
