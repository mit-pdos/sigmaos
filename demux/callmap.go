package demux

import (
	"fmt"
	"sync"

	"sigmaos/frame"
	"sigmaos/serr"
	"sigmaos/sessp"
)

// One call struct per outstanding call, which consists of a request
// or reply, which in turns are a slice of frames.
type call struct {
	ch      chan *serr.Err
	seqno   sessp.Tseqno
	request []frame.Tframe
	reply   []frame.Tframe
}

func (r *call) String() string {
	return fmt.Sprintf("{call %d %d %d}", r.seqno, len(r.request), len(r.reply))
}

// Map of outstanding calls indexed by sequence number
type callMap struct {
	sync.Mutex
	closed bool
	calls  map[sessp.Tseqno]*call
}

func newCallMap() *callMap {
	return &callMap{calls: make(map[sessp.Tseqno]*call)}
}

func (cm *callMap) close() error {
	cm.Lock()
	defer cm.Unlock()

	cm.closed = true
	return nil
}

func (cm *callMap) isClosed() bool {
	cm.Lock()
	defer cm.Unlock()

	return cm.closed
}

func (cm *callMap) outstanding() []sessp.Tseqno {
	cm.Lock()
	defer cm.Unlock()

	o := make([]sessp.Tseqno, 0, len(cm.calls))
	for k, _ := range cm.calls {
		o = append(o, k)
	}
	return o
}

func (cm *callMap) put(seqno sessp.Tseqno, call *call) *serr.Err {
	cm.Lock()
	defer cm.Unlock()
	if cm.closed {
		return serr.NewErr(serr.TErrUnreachable, "dmxclnt")
	}
	cm.calls[seqno] = call
	return nil
}

func (cm *callMap) remove(seqno sessp.Tseqno) (*call, bool) {
	cm.Lock()
	defer cm.Unlock()

	last := false
	if call, ok := cm.calls[seqno]; ok {
		delete(cm.calls, seqno)
		if len(cm.calls) == 0 && cm.closed {
			last = true
		}
		return call, last
	}
	return nil, last
}
