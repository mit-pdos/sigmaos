package sessionclnt

import (
	"strings"
	//	"sync"
	"github.com/sasha-s/go-deadlock"

	db "ulambda/debug"
	np "ulambda/ninep"
)

type ClntSessMgr struct {
	mu       deadlock.Mutex
	sid      np.Tsession
	seqno    *np.Tseqno
	sessions map[string]*clntsession // XXX Is a ClntSessMgr ever used to talk to multiple servers?
}

func MakeClntSessMgr(session np.Tsession, seqno *np.Tseqno) *ClntSessMgr {
	sc := &ClntSessMgr{}
	sc.sessions = make(map[string]*clntsession)
	sc.sid = session
	sc.seqno = seqno
	return sc
}

func (sc *ClntSessMgr) Exit() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	db.DLPrintf("SESSCLNT", "Exit\n")

	for addr, sess := range sc.sessions {
		db.DLPrintf("SESSCLNT", "exit close session to %v\n", addr)
		sess.close()
		delete(sc.sessions, addr)
	}
}

// Return an existing sess if there is one, else allocate a new one. Caller
// holds lock.
func (sc *ClntSessMgr) allocConn(addrs []string) (*clntsession, *np.Err) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	// Store as concatenation of addresses
	key := sessKey(addrs)
	if sess, ok := sc.sessions[key]; ok {
		return sess, nil
	}
	sess, err := makeConn(sc.sid, sc.seqno, addrs)
	if err != nil {
		return nil, err
	}
	sc.sessions[key] = sess
	return sess, nil
}

func (sc *ClntSessMgr) RPC(addrs []string, req np.Tmsg, f np.Tfence1) (np.Tmsg, *np.Err) {
	db.DLPrintf("SESSCLNT", "%v RPC %v %v to %v\n", sc.sid, req.Type(), req, addrs)
	// Get or establish sessection
	sess, err := sc.allocConn(addrs)
	if err != nil {
		db.DLPrintf("SESSCLNT", "%v Unable to alloc sess for req %v %v err %v to %v\n", req.Type(), req, err, addrs)
		return nil, err
	}
	rpc, err := sess.send(req, f)
	if err != nil {
		db.DLPrintf("SESSCLNT", "%v Unable to send req %v %v err %v to %v\n", sc.sid, req.Type(), req, err, addrs)
		return nil, err
	}

	// Reliably receive a response from one of the replicas.
	reply, err := sess.recv(rpc)
	if err != nil {
		db.DLPrintf("SESSCLNT", "%v Unable to recv response to req %v %v err %v from %v\n", sc.sid, req.Type(), req, err, addrs)
		return nil, err
	}
	return reply, nil
}

func (sc *ClntSessMgr) Disconnect(addrs []string) *np.Err {
	db.DLPrintf("SESSCLNT", "%v Disconnect %v\n", sc.sid, addrs)
	key := sessKey(addrs)
	sc.mu.Lock()
	sess, ok := sc.sessions[key]
	sc.mu.Unlock()
	if !ok {
		return np.MkErr(np.TErrUnreachable, sessKey(addrs))
	}
	sess.close()
	return nil
}

func sessKey(addrs []string) string {
	return strings.Join(addrs, ",")
}
