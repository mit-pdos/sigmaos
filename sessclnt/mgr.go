package sessclnt

import (
	"strings"
	"sync"
	//	"github.com/sasha-s/go-deadlock"

	db "ulambda/debug"
	np "ulambda/ninep"
)

type Mgr struct {
	mu       sync.Mutex
	sid      np.Tsession
	seqno    *np.Tseqno
	sessions map[string]*sessClnt
}

func MakeMgr(session np.Tsession, seqno *np.Tseqno) *Mgr {
	sc := &Mgr{}
	sc.sessions = make(map[string]*sessClnt)
	sc.sid = session
	sc.seqno = seqno
	return sc
}

func (sc *Mgr) Exit() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	db.DPrintf("SESSCLNT", "Exit\n")

	for addr, sess := range sc.sessions {
		db.DPrintf("SESSCLNT", "exit close session to %v\n", addr)
		sess.close()
		delete(sc.sessions, addr)
	}
}

// Return an existing sess if there is one, else allocate a new one. Caller
// holds lock.
func (sc *Mgr) allocSessClnt(addrs []string) (*sessClnt, *np.Err) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	// Store as concatenation of addresses
	key := sessKey(addrs)
	if sess, ok := sc.sessions[key]; ok {
		return sess, nil
	}
	sess, err := makeSessClnt(sc.sid, sc.seqno, addrs)
	if err != nil {
		return nil, err
	}
	sc.sessions[key] = sess
	return sess, nil
}

func (sc *Mgr) RPC(addrs []string, req np.Tmsg, f np.Tfence) (np.Tmsg, *np.Err) {
	db.DPrintf("SESSCLNT", "%v RPC %v %v to %v\n", sc.sid, req.Type(), req, addrs)
	// Get or establish sessection
	sess, err := sc.allocSessClnt(addrs)
	if err != nil {
		db.DPrintf("SESSCLNT", "Unable to alloc sess for req %v %v err %v to %v\n", req.Type(), req, err, addrs)
		return nil, err
	}
	return sess.rpc(req, f)
}

func (sc *Mgr) Disconnect(addrs []string) *np.Err {
	db.DPrintf("SESSCLNT", "Disconnect %v %v\n", sc.sid, addrs)
	key := sessKey(addrs)
	sc.mu.Lock()
	sess, ok := sc.sessions[key]
	sc.mu.Unlock()
	if !ok {
		return np.MkErr(np.TErrUnreachable, sessKey(addrs))
	}
	sess.sessClose()
	return nil
}

func sessKey(addrs []string) string {
	return strings.Join(addrs, ",")
}
