package protclnt

import (
	"strings"
	"sync"

	db "ulambda/debug"
	"ulambda/fences"
	"ulambda/netclnt"
	np "ulambda/ninep"
)

// XXX duplicate
const (
	Msglen = 64 * 1024
)

type conn struct {
	nc *netclnt.NetClnt
	fm *fences.FenceTable
}

func makeConn(nc *netclnt.NetClnt) *conn {
	c := &conn{}
	c.fm = fences.MakeFenceTable()
	c.nc = nc
	return c
}

func (conn *conn) send(req np.Tmsg, session np.Tsession, seqno *np.Tseqno) (np.Tmsg, *np.Err) {
	reqfc := np.MakeFcall(req, session, seqno)
	repfc, err := conn.nc.RPC(reqfc)
	if err != nil {
		return nil, err
	}
	return repfc.Msg, nil
}

// XXX SessMgr?
type ConnMgr struct {
	mu      sync.Mutex
	session np.Tsession
	seqno   *np.Tseqno
	conns   map[string]*conn
}

func makeConnMgr(session np.Tsession, seqno *np.Tseqno) *ConnMgr {
	cm := &ConnMgr{}
	cm.conns = make(map[string]*conn)
	cm.session = session
	cm.seqno = seqno
	return cm
}

func (cm *ConnMgr) exit() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for addr, conn := range cm.conns {
		db.DLPrintf("9PCHAN", "exit close connection to %v\n", addr)
		conn.nc.Close()
		delete(cm.conns, addr)
	}
}

// XXX Make array
func (cm *ConnMgr) allocConn(addrs []string) (*conn, *np.Err) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Store as concatenation of addresses
	key := strings.Join(addrs, ".")

	if conn, ok := cm.conns[key]; ok {
		return conn, nil
	}
	nc, err := netclnt.MkNetClnt(addrs)
	if err != nil {
		return nil, err
	}
	cm.conns[key] = makeConn(nc)
	return cm.conns[key], nil
}

func (cm *ConnMgr) lookupConn(addrs []string) (*conn, bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	conn, ok := cm.conns[strings.Join(addrs, ",")]
	return conn, ok
}

func (cm *ConnMgr) makeCall(dst []string, req np.Tmsg) (np.Tmsg, *np.Err) {
	conn, err := cm.allocConn(dst)
	if err != nil {
		return nil, err
	}
	return conn.send(req, cm.session, cm.seqno)
}

func (cm *ConnMgr) disconnect(dst []string) *np.Err {
	conn, ok := cm.lookupConn(dst)
	if !ok {
		return np.MkErr(np.TErrUnreachable, strings.Join(dst, "."))
	}
	conn.nc.Close()
	return nil
}
