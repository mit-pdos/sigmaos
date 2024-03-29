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
	cid   sp.TclntId
	seqno sp.Tseqno
}

func NewReplClnt(fsls []*fslib.FsLib) *ReplClnt {
	rc := &ReplClnt{
		cid:       fsls[0].ClntId(),
		ClntCache: rpcclnt.NewRPCClntCache(fsls),
	}
	return rc
}

func (rc *ReplClnt) nextSeqno() sp.Tseqno {
	seq := &rc.seqno
	return seq.Next()
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
