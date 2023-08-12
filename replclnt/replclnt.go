package replclnt

import (
	"google.golang.org/protobuf/proto"

	"sigmaos/fslib"
	replproto "sigmaos/repl/proto"
	"sigmaos/rpcclnt"
	sp "sigmaos/sigmap"
)

type ReplClnt struct {
	*rpcclnt.ClntCache
}

func NewReplClnt(fsls []*fslib.FsLib) *ReplClnt {
	rc := &ReplClnt{ClntCache: rpcclnt.NewRPCClntCache(fsls)}
	return rc
}

func NewReplOp(method, key string, cid sp.TclntId, seqno sp.Tseqno, val proto.Message) (*replproto.ReplOpRequest, error) {
	b, err := proto.Marshal(val)
	if err != nil {
		return nil, err
	}
	return &replproto.ReplOpRequest{
		Method: method,
		ClntId: uint32(cid),
		Seqno:  uint64(seqno),
		Msg:    b,
	}, nil
}

func (rc *ReplClnt) ReplOp(srv, method, key string, cid sp.TclntId, seqno sp.Tseqno, val proto.Message) ([]byte, error) {
	req, err := NewReplOp(method, key, cid, seqno, val)
	if err != nil {
		return nil, err
	}
	var res replproto.ReplOpReply
	if err := rc.RPC(srv, "ReplSrv.ProcessOp", req, &res); err != nil {
		return nil, err
	}
	return res.Msg, nil
}
