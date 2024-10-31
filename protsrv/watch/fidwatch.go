package watch

import (
	"fmt"
	db "sigmaos/debug"
	"sigmaos/protsrv/pobj"
	"sync"
)

type FidWatch struct {
	mu     *sync.Mutex
	po     *pobj.Pobj
	events []string
	unfinishedEvent string
	cond   sync.Cond
	watch  *WatchV2
}

func NewFidWatch(pobj *pobj.Pobj) *FidWatch {
	mu := sync.Mutex{}
	db.DPrintf(db.WATCH_V2, "NewFidWatch '%v'\n", pobj.Pathname())
	return &FidWatch{&mu, pobj, nil, "", *sync.NewCond(&mu), nil}
}

func (f *FidWatch) String() string {
	return fmt.Sprintf("{po %v ev %v ufev %s}", f.po, f.events, f.unfinishedEvent)
}

func (f *FidWatch) Pobj() *pobj.Pobj {
	return f.po
}

func (f *FidWatch) Watch() *WatchV2 {
	return f.watch
}

func (f *FidWatch) Close() {
	f.mu.Lock()
	defer f.mu.Unlock()
}

// creates a buffer with as many events as possible, blocking if there are currently no events
func (f *FidWatch) GetEventBuffer(maxLength int) []byte {
	f.mu.Lock()
	defer f.mu.Unlock()

	for len(f.events) == 0 && f.unfinishedEvent == "" {
		f.cond.Wait()
	}

	buf := make([]byte, 0)
	offset := uint32(0)

	if f.unfinishedEvent != "" {
		var offsetDiff int

		buf, f.unfinishedEvent, offsetDiff = f.addEvent(buf, maxLength, f.unfinishedEvent)
		offset += uint32(offsetDiff)
	}

	if f.unfinishedEvent == "" {
		maxIxReached := -1
		for ix, event := range f.events {
			db.DPrintf(db.WATCH_V2, "ReadFWatch event %v\n", event)
			eventStr := event + "\n"
			var offsetDiff int
			buf, f.unfinishedEvent, offsetDiff = f.addEvent(buf, maxLength - int(offset), eventStr)
			offset += uint32(offsetDiff)

			maxIxReached = ix
			if f.unfinishedEvent != "" {
				break
			} 
		}

		f.events = f.events[maxIxReached + 1:]
	}

	return buf
}

func (f *FidWatch) addEvent(buffer []byte, remainingCapacity int, event string) ([]byte, string, int) {
	if remainingCapacity < len(event) {
		buffer = append(buffer, event[:remainingCapacity]...)
		return buffer, event[remainingCapacity:], remainingCapacity
	}

	buffer = append(buffer, event...)
	return buffer, "", len(event)
}
