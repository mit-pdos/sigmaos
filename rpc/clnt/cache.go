// The rpcclnt package caches RPC clnts to avoid repeated lookups
package clnt

import (
	"sync"
	"sync/atomic"

	protobuf "google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/rpc"
	rpcclntopts "sigmaos/rpc/clnt/opts"
	"sigmaos/serr"
	"sigmaos/util/retry"
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

// Retry lookup if rpcc.RPC cannot reach server.
func (cc *ClntCache) RPCRetry(pn string, method string, arg protobuf.Message, res protobuf.Message) error {
	var err error
	var rpcc *RPCClnt
	for i := 0; i < 2; i++ {
		rpcc, err = cc.Lookup(pn)
		if err != nil {
			db.DPrintf(db.RPCCLNT, "RPCRetry lookup pn %v %v err %v", pn, method, err)
			return err
		}
		err = rpcc.RPC(method, arg, res)
		if err == nil {
			return nil
		}
		if !serr.IsErrorWalkOK(err) {
			db.DPrintf(db.RPCCLNT, "RPCRetry RPC %v %v err %v", pn, method, err)
			return err
		}
		db.DPrintf(db.RPCCLNT, "RPCRetry retry %v %v err %v", pn, method, err)
		cc.Delete(pn)
		cc.stats.Nretry.Add(1)
	}
	return err
}

func (cc *ClntCache) RPC(pn string, method string, arg protobuf.Message, res protobuf.Message) error {
	return cc.RPCRetry(pn, method, arg, res)
}

func (cc *ClntCache) RPCRetryNotFound(pn, notfound, method string, arg protobuf.Message, res protobuf.Message) error {
	err, ok := retry.RetryDefDur(func() error {
		err := cc.RPC(pn, method, arg, res)
		if err != nil {
			db.DPrintf(db.RPCCLNT, "RPCRetryNotFound: rpc %v (%v) err %v", pn, notfound, err)
		}
		return err
	}, func(err error) bool {
		// if not found, try again, because srv may not have started yet,
		// or a new srv may start and take over,
		return serr.IsErrorNotfound(err) && err.(*serr.Err).Obj == notfound
	})
	if !ok {
		return serr.NewErr(serr.TErrUnreachable, pn)
	}
	return err
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
