package rpcclnt

import (
	"errors"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/pathclnt"
	"sigmaos/rpc"
	"sigmaos/serr"
)

//
//  Cache of RPC clnts to avoid repeated lookups
//

type ClntCache struct {
	sync.Mutex
	fsls  []*fslib.FsLib
	rpccs map[string]*RPCClnt
}

func NewRPCClntCache(fsls []*fslib.FsLib) *ClntCache {
	return &ClntCache{fsls: fsls, rpccs: make(map[string]*RPCClnt)}
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
	rpcc, err := MkRPCClnt(cc.fsls, pn)
	cc.Lock()
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
	for i := 0; i < pathclnt.MAXRETRY; i++ {
		rpcc, err := cc.Lookup(pn)
		if err != nil {
			return err
		}
		if err := rpcc.RPC(method, arg, res); err == nil {
			return nil
		} else {
			var sr *serr.Err
			if errors.As(err, &sr) && pathclnt.Retry(sr) {
				time.Sleep(pathclnt.TIMEOUT * time.Millisecond)
				db.DPrintf(db.ALWAYS, "RPC: retry %v %v err %v\n", pn, method, sr)
				cc.Delete(pn)
				continue
			}
			return err
		}
	}
	return serr.MkErr(serr.TErrUnreachable, pn)
}

func (cc *ClntCache) RPC(pn string, method string, arg proto.Message, res proto.Message) error {
	return cc.RPCRetry(pn, method, arg, res)
}

func (cc *ClntCache) StatsSrv(pn string) (*rpc.SigmaRPCStats, error) {
	rpcc, err := cc.Lookup(pn)
	if err != nil {
		return nil, err
	}
	return rpcc.StatsSrv()
}

func (cc *ClntCache) StatsClnt() []map[string]*rpc.MethodStat {
	stats := make([]map[string]*rpc.MethodStat, 0, len(cc.rpccs))
	for _, rpcc := range cc.rpccs {
		stats = append(stats, rpcc.StatsClnt())
	}
	return stats
}
