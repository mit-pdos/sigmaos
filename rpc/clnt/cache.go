// The rpcclnt package caches RPC clnts to avoid repeated lookups
package clnt

import (
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/rpc"
	rpcclntopts "sigmaos/rpc/clnt/opts"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type ClntCache struct {
	sync.Mutex
	rpccs   map[string]*RPCClnt
	rpcOpts *rpcclntopts.RPCClntOptions
	stats   Tstats
}

type Tstats struct {
	Nretry atomic.Uint64
}

func NewRPCClntCache(opts ...*rpcclntopts.RPCClntOption) *ClntCache {
	rpcOpts := rpcclntopts.NewEmptyRPCClntOptions()
	for _, opt := range opts {
		opt.Apply(rpcOpts)
	}
	return &ClntCache{
		rpccs:   make(map[string]*RPCClnt),
		rpcOpts: rpcOpts,
	}
}

func (cc *ClntCache) Stats() Tstats {
	return cc.stats
}

// Note: several threads may call Lookup for same pn, overwriting the
// rpcc of the last thread that called Lookup.
func (cc *ClntCache) Lookup(pn string) (*RPCClnt, error) {
	cc.Lock()
	defer cc.Unlock()
	rpcc, ok := cc.rpccs[pn]
	if ok {
		return rpcc, nil
	}
	cc.Unlock()
	ch, err := cc.rpcOpts.NewRPCChannel(pn)
	cc.Lock()
	if err != nil {
		return nil, err
	}
	rpcc, err = NewRPCClnt(pn, rpcclntopts.WithRPCChannel(ch))
	if err != nil {
		return nil, err
	}
	cc.rpccs[pn] = rpcc
	return rpcc, nil
}

func (cc *ClntCache) Delete(pn string) {
	cc.Lock()
	defer cc.Unlock()
	delete(cc.rpccs, pn)
}

func (cc *ClntCache) RPCRetry(pn string, method string, arg proto.Message, res proto.Message) error {
	for i := 0; i < sp.Conf.Path.MAX_RESOLVE_RETRY; i++ {
		rpcc, err := cc.Lookup(pn)
		if err != nil {
			var sr *serr.Err
			if errors.As(err, &sr) && serr.IsRetryOK(sr) {
				db.DPrintf(db.RPCCLNT, "RPC retry lookup failure pn %v", pn)
				continue
			}
			db.DPrintf(db.RPCCLNT, "RPC lookup failure: give up pn %v", pn)
			return err
		}
		if err := rpcc.RPC(method, arg, res); err != nil {
			var sr *serr.Err
			if errors.As(err, &sr) && serr.IsRetryOK(sr) {
				time.Sleep(sp.Conf.Path.RESOLVE_TIMEOUT)
				cc.stats.Nretry.Add(1)
				db.DPrintf(db.RPCCLNT, "RPC: retry %v %v err %v", pn, method, sr)
				cc.Delete(pn)
				continue
			}
			db.DPrintf(db.RPCCLNT, "RPCRetry no retry %v err: %v sr: %v", pn, err, sr)
			return err
		}
		return nil
	}
	return serr.NewErr(serr.TErrUnreachable, pn)
}

func (cc *ClntCache) RPC(pn string, method string, arg proto.Message, res proto.Message) error {
	return cc.RPCRetry(pn, method, arg, res)
}

func (cc *ClntCache) StatsSrv(pn string) (*rpc.RPCStatsSnapshot, error) {
	rpcc, err := cc.Lookup(pn)
	if err != nil {
		return nil, err
	}
	return rpcc.StatsSrv()
}

func (cc *ClntCache) StatsClnt() []map[string]*rpc.MethodStatSnapshot {
	stats := make([]map[string]*rpc.MethodStatSnapshot, 0, len(cc.rpccs))
	for _, rpcc := range cc.rpccs {
		stats = append(stats, rpcc.StatsClnt())
	}
	return stats
}
