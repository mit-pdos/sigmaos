package npclnt

import (
	"sync"

	// db "ulambda/debug"
	np "ulambda/ninep"
)

// XXX duplicate
const (
	Msglen = 64 * 1024
)

type ChanMgr struct {
	mu    sync.Mutex
	conns map[string]*Chan
}

func makeChanMgr() *ChanMgr {
	cm := &ChanMgr{}
	cm.conns = make(map[string]*Chan)
	return cm
}

func (cm *ChanMgr) lookup(addr string) (*Chan, bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	conn, ok := cm.conns[addr]
	return conn, ok
}

func (cm *ChanMgr) allocChan(addr string) (*Chan, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	var err error
	conn, ok := cm.conns[addr]
	if !ok {
		conn, err = mkChan(addr)
		if err == nil {
			cm.conns[addr] = conn
		}
	}
	return conn, err
}

func (cm *ChanMgr) Close(addr string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	conn, ok := cm.conns[addr]
	if ok {
		conn.Close()
		delete(cm.conns, addr)
	}
}

func (cm *ChanMgr) makeCall(addr string, req np.Tmsg) (np.Tmsg, error) {
	conn, err := cm.allocChan(addr)
	if err != nil {
		return nil, err
	}
	reqfc := &np.Fcall{}
	reqfc.Type = req.Type()
	reqfc.Msg = req
	repfc, err := conn.RPC(reqfc)
	if err != nil {
		return nil, err
	}
	return repfc.Msg, nil
}
