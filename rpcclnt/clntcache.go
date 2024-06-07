// The rpcclnt package caches RPC clnts to avoid repeated lookups
package rpcclnt

import (
	"errors"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/rpc"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type ClntCache struct {
	sync.Mutex
	rpccs   map[string]*RPCClnt
	newChFn NewRPCChFn
}

func NewRPCClntCache(fn NewRPCChFn) *ClntCache {
	return &ClntCache{
		rpccs:   make(map[string]*RPCClnt),
		newChFn: fn,
	}
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
	ch, err := cc.newChFn(pn)
	cc.Lock()
	if err != nil {
		return nil, err
	}
	rpcc = NewRPCClnt(ch)
	cc.rpccs[pn] = rpcc
	return rpcc, nil
}

func (cc *ClntCache) Delete(pn string) {
	cc.Lock()
	defer cc.Unlock()
	delete(cc.rpccs, pn)
}

func (cc *ClntCache) RPCRetry(pn string, method string, arg proto.Message, res proto.Message) error {
	for i := 0; i < sp.PATHCLNT_MAXRETRY; i++ {
		rpcc, err := cc.Lookup(pn)
		if err != nil {
			var sr *serr.Err
			if errors.As(err, &sr) && serr.Retry(sr) {
				db.DPrintf(db.RPCCLNT, "RPC retry lookup failure pn %v", pn)
				continue
			}
			db.DPrintf(db.RPCCLNT, "RPC lookup failure: give up pn %v", pn)
			return err
		}
		if err := rpcc.RPC(method, arg, res); err != nil {
			var sr *serr.Err
			if errors.As(err, &sr) && serr.Retry(sr) {
				time.Sleep(sp.PATHCLNT_TIMEOUT * time.Millisecond)
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
