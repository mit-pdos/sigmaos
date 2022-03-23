package sessionclnt

import (
	"strings"
	//	"sync"
	"github.com/sasha-s/go-deadlock"

	db "ulambda/debug"
	"ulambda/netclnt"
	np "ulambda/ninep"
)

type SessClnt struct {
	mu    deadlock.Mutex
	sid   np.Tsession
	seqno *np.Tseqno
	conns map[string]*conn // XXX Is a SessClnt ever used to talk to multiple servers?
}

func MakeSessClnt(session np.Tsession, seqno *np.Tseqno) *SessClnt {
	sc := &SessClnt{}
	sc.conns = make(map[string]*conn)
	sc.sid = session
	sc.seqno = seqno
	return sc
}

func (sc *SessClnt) Exit() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	db.DLPrintf("SESSCLNT", "Exit\n")

	for addr, conn := range sc.conns {
		db.DLPrintf("SESSCLNT", "exit close connection to %v\n", addr)
		conn.close()
		delete(sc.conns, addr)
	}
}

// Return an existing conn if there is one, else allocate a new one. Caller
// holds lock.
func (sc *SessClnt) allocConn(addrs []string) (*conn, *np.Err) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	// Store as concatenation of addresses
	key := connKey(addrs)
	if conn, ok := sc.conns[key]; ok {
		return conn, nil
	}
	conn, err := makeConn(sc.sid, sc.seqno, addrs)
	if err != nil {
		return nil, err
	}
	sc.conns[key] = conn
	return conn, nil
}

func (sc *SessClnt) RPC(addrs []string, req np.Tmsg, f np.Tfence1) (np.Tmsg, *np.Err) {
	db.DLPrintf("SESSCLNT", "%v RPC %v %v to %v\n", sc.sid, req.Type(), req, addrs)
	// Get or establish connection
	conn, err := sc.allocConn(addrs)
	if err != nil {
		db.DLPrintf("SESSCLNT", "%v Unable to alloc conn for req %v %v err %v to %v\n", req.Type(), req, err, addrs)
		return nil, err
	}
	rpc, err := sc.atomicSend(conn, req, f)
	if err != nil {
		db.DLPrintf("SESSCLNT", "%v Unable to send req %v %v err %v to %v\n", sc.sid, req.Type(), req, err, addrs)
		return nil, err
	}

	// Reliably receive a response from one of the replicas.
	reply, err := conn.recv(rpc)
	if err != nil {
		db.DLPrintf("SESSCLNT", "%v Unable to recv response to req %v %v err %v from %v\n", sc.sid, req.Type(), req, err, addrs)
		return nil, err
	}
	return reply, nil
}

// Atomically allocate a seqno and try to send.
func (sc *SessClnt) atomicSend(conn *conn, req np.Tmsg, f np.Tfence1) (*netclnt.Rpc, *np.Err) {
	// Take the lock to ensure requests are sent in order of seqno.
	sc.mu.Lock()
	defer sc.mu.Unlock()
	rpc := netclnt.MakeRpc(np.MakeFcall(req, sc.sid, sc.seqno, f))
	// Reliably send the RPC to a replica. If the replica becomes unavailable,
	// this request will be resent.
	if err := conn.send(rpc); err != nil {
		return nil, err
	}
	return rpc, nil
}

func (sc *SessClnt) Disconnect(addrs []string) *np.Err {
	db.DLPrintf("SESSCLNT", "%v Disconnect %v\n", sc.sid, addrs)
	key := connKey(addrs)
	sc.mu.Lock()
	conn, ok := sc.conns[key]
	sc.mu.Unlock()
	if !ok {
		return np.MkErr(np.TErrUnreachable, connKey(addrs))
	}
	conn.close()
	return nil
}

func connKey(addrs []string) string {
	return strings.Join(addrs, ",")
}
