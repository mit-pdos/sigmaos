package cachereplclnt

import (
	"google.golang.org/protobuf/proto"

	"sigmaos/cacheclnt"
	"sigmaos/fslib"
	replproto "sigmaos/repl/proto"
	sp "sigmaos/sigmap"
)

type CacheReplClnt struct {
	*cacheclnt.CacheClnt
}

func NewCacheReplClnt(fsls []*fslib.FsLib, job string, nshard int) *CacheReplClnt {
	cc := &CacheReplClnt{CacheClnt: cacheclnt.NewCacheClnt(fsls, job, nshard)}
	return cc
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

func (cc *CacheReplClnt) ReplOpSrv(srv, method, key string, cid sp.TclntId, seqno sp.Tseqno, val proto.Message) ([]byte, error) {
	req, err := NewReplOp(method, key, cid, seqno, val)
	if err != nil {
		return nil, err
	}
	var res replproto.ReplOpReply
	if err := cc.RPC(srv, "CacheSrvRepl.ProcessOp", req, &res); err != nil {
		return nil, err
	}
	return res.Msg, nil
}
