package npclnt

import (
	"strings"
	"sync"

	db "ulambda/debug"
	"ulambda/netclnt"
	np "ulambda/ninep"
)

// XXX duplicate
const (
	Msglen = 64 * 1024
)

type ConnMgr struct {
	mu      sync.Mutex
	name    string
	session np.Tsession
	seqno   *np.Tseqno
	conns   map[string]*netclnt.NetClnt
}

func makeConnMgr(session np.Tsession, seqno *np.Tseqno) *ConnMgr {
	cm := &ConnMgr{}
	cm.conns = make(map[string]*netclnt.NetClnt)
	cm.session = session
	cm.seqno = seqno
	return cm
}

func (cm *ConnMgr) exit() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for addr, conn := range cm.conns {
		db.DLPrintf("9PCHAN", "exit close connection to %v\n", addr)
		conn.Close()
		delete(cm.conns, addr)
	}
}

// XXX Make array
func (cm *ConnMgr) allocConn(addrs []string) (*netclnt.NetClnt, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Store as concatenation of addresses
	key := strings.Join(addrs, ",")

	var err error
	conn, ok := cm.conns[key]
	if !ok {
		conn, err = netclnt.MkNetClnt(addrs)
		if err == nil {
			cm.conns[key] = conn
		}
	}
	return conn, err
}

func (cm *ConnMgr) makeCall(dst []string, req np.Tmsg) (np.Tmsg, error) {
	conn, err := cm.allocConn(dst)
	if err != nil {
		return nil, err
	}
	reqfc := &np.Fcall{}
	reqfc.Type = req.Type()
	reqfc.Msg = req
	reqfc.Session = cm.session
	reqfc.Seqno = cm.seqno.Next()
	repfc, err := conn.RPC(reqfc)
	if err != nil {
		return nil, err
	}
	return repfc.Msg, nil
}
