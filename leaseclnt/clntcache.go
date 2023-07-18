package leaseclnt

import (
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	db "sigmaos/debug"
	"sigmaos/fslib"
	"sigmaos/pathclnt"
	"sigmaos/protdevclnt"
	"sigmaos/serr"
)

type ClntCache struct {
	sync.Mutex
	fsl *fslib.FsLib
	cc  map[string]*protdevclnt.ProtDevClnt
}

func NewClntCache(fsl *fslib.FsLib) *ClntCache {
	return &ClntCache{fsl: fsl, cc: make(map[string]*protdevclnt.ProtDevClnt)}
}

// Note: several threads may call MkProtDevClnt for same pn,
// overwriting the pdc of the last thread that called NewClnt.
func (cc *ClntCache) Lookup(pn string) (*protdevclnt.ProtDevClnt, error) {
	cc.Lock()
	defer cc.Unlock()
	pdc, ok := cc.cc[pn]
	if ok {
		return pdc, nil
	}
	cc.Unlock()
	pdc, err := protdevclnt.MkProtDevClnt([]*fslib.FsLib{cc.fsl}, pn)
	cc.Lock()
	if err != nil {
		return nil, err
	}
	cc.cc[pn] = pdc
	return pdc, nil
}

func (cc *ClntCache) Delete(pn string) {
	cc.Lock()
	defer cc.Unlock()
	delete(cc.cc, pn)
}

func (cc *ClntCache) RPC(pn string, method string, arg proto.Message, res proto.Message) error {
	for i := 0; i < pathclnt.MAXRETRY; i++ {
		pdc, err := cc.Lookup(pn)
		if err != nil {
			return err
		}
		if err := pdc.RPC(method, arg, res); err == nil {
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
