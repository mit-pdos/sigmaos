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
	name  string
	conns map[string]*Chan
}

func makeChanMgr(name string) *ChanMgr {
	cm := &ChanMgr{}
	cm.conns = make(map[string]*Chan)
	cm.name = name
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
		conn, err = mkChan(cm.name, addr)
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

func (cm *ChanMgr) makeCall(src, dst string, req np.Tmsg) (np.Tmsg, error) {
	conn, err := cm.allocChan(dst)
	if err != nil {
		return nil, err
	}
	reqfc := &np.Fcall{}
	reqfc.Type = req.Type()
	reqfc.Msg = req
	repfc, err := conn.RPC(src, reqfc)
	if err != nil {
		return nil, err
	}
	return repfc.Msg, nil
}
