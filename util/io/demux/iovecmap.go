package demux

import (
	"sync"

	"sigmaos/serr"
	sessp "sigmaos/session/proto"
)

// Map of outstanding calls indexed by tag
type IoVecMap struct {
	sync.Mutex
	closed bool
	calls  map[sessp.Ttag]*sessp.IoVec
}

func NewIoVecMap() *IoVecMap {
	return &IoVecMap{
		calls: make(map[sessp.Ttag]*sessp.IoVec),
	}
}

func (cm *IoVecMap) close() error {
	cm.Lock()
	defer cm.Unlock()

	cm.closed = true
	return nil
}

func (cm *IoVecMap) isClosed() bool {
	cm.Lock()
	defer cm.Unlock()

	return cm.closed
}

func (cm *IoVecMap) Put(tag sessp.Ttag, iov *sessp.IoVec) *serr.Err {
	cm.Lock()
	defer cm.Unlock()
	if cm.closed {
		return serr.NewErr(serr.TErrUnreachable, "dmxclnt")
	}
	cm.calls[tag] = iov
	return nil
}

func (cm *IoVecMap) Get(tag sessp.Ttag) (*sessp.IoVec, bool) {
	cm.Lock()
	defer cm.Unlock()

	if iov, ok := cm.calls[tag]; ok {
		delete(cm.calls, tag)
		return iov, true
	}
	return nil, false
}
