package demux

import (
	"sync"

	"sigmaos/serr"
	sessp "sigmaos/session/proto"
)

// Map of outstanding calls indexed by tag
type callMap struct {
	sync.Mutex
	closed bool
	calls  map[sessp.Ttag]chan reply
}

func newCallMap() *callMap {
	return &callMap{calls: make(map[sessp.Ttag]chan reply)}
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

func (cm *callMap) put(tag sessp.Ttag, ch chan reply) *serr.Err {
	cm.Lock()
	defer cm.Unlock()

	if cm.closed {
		return serr.NewErr(serr.TErrUnreachable, "dmxclnt")
	}
	cm.calls[tag] = ch
	return nil
}

func (cm *callMap) remove(tag sessp.Ttag) (chan reply, bool) {
	cm.Lock()
	defer cm.Unlock()

	if ch, ok := cm.calls[tag]; ok {
		delete(cm.calls, tag)
		return ch, true
	}
	return nil, false
}

func (cm *callMap) outstanding() []sessp.Ttag {
	cm.Lock()
	defer cm.Unlock()

	ts := make([]sessp.Ttag, 0, len(cm.calls))
	for t, _ := range cm.calls {
		ts = append(ts, t)
	}
	return ts
}
