package demux

import (
	"io"

	"sigmaos/frame"
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

type DemuxClnt struct {
	out    io.Writer
	in     io.Reader
	reqmap *reqMap
}

func NewDemuxClnt(out io.Writer, in io.Reader) *DemuxClnt {
	dmx := &DemuxClnt{out, in, newReqMap()}
	return dmx
}

func (dmx *DemuxClnt) SendReceive(a []byte) ([]byte, error) {
	if err := frame.WriteFrame(dmx.out, a); err != nil {
		return nil, err
	}
	b, r := frame.ReadFrame(dmx.in)
	if r != nil {
		return nil, r
	}
	return b, nil
}
