package leaseclnt

import (
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/pathclnt"
	"sigmaos/rpcclnt"
	"sigmaos/serr"
)

type ClntCache struct {
	sync.Mutex
	fsl *fslib.FsLib
	cc  map[string]*rpcclnt.RPCClnt
}

func NewClntCache(fsl *fslib.FsLib) *ClntCache {
	return &ClntCache{fsl: fsl, cc: make(map[string]*rpcclnt.RPCClnt)}
}

// Note: several threads may call MkRPCClnt for same pn,
// overwriting the pdc of the last thread that called NewClnt.
func (cc *ClntCache) Lookup(pn string) (*rpcclnt.RPCClnt, error) {
	cc.Lock()
	defer cc.Unlock()
	rpcc, ok := cc.cc[pn]
	if ok {
		return rpcc, nil
	}
	cc.Unlock()
	rpcc, err := rpcclnt.MkRPCClnt([]*fslib.FsLib{cc.fsl}, pn)
	cc.Lock()
	if err != nil {
		return nil, err
	}
	cc.cc[pn] = rpcc
	return rpcc, nil
}

func (cc *ClntCache) Delete(pn string) {
	cc.Lock()
	defer cc.Unlock()
	delete(cc.cc, pn)
}

func (cc *ClntCache) RPC(pn string, method string, arg proto.Message, res proto.Message) error {
	for i := 0; i < pathclnt.MAXRETRY; i++ {
		rpcc, err := cc.Lookup(pn)
		if err != nil {
			return err
		}
		if err := rpcc.RPC(method, arg, res); err == nil {
			return nil
		} else {
			cc.Delete(pn)
			if serr.IsErrCode(err, serr.TErrUnreachable) {
				time.Sleep(pathclnt.TIMEOUT * time.Millisecond)
				db.DPrintf(db.ALWAYS, "RPC: retry %v %v\n", pn, method)
				continue
			}
			return err
		}
	}
	return serr.MkErr(serr.TErrUnreachable, pn)
}
