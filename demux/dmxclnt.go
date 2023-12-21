package demux

import (
	sp "sigmaos/sigmap"
	"sync"
)

type req struct {
	ch chan error
}

type reqMap struct {
	sync.Mutex
	reqs map[sp.Tseqno]*req
}

func newReqMap() *reqMap {
	return &reqMap{reqs: make(map[sp.Tseqno]*req)}
}
