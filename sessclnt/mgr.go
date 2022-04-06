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
	sessions map[string]*SessClnt
}

func MakeMgr(session np.Tsession, seqno *np.Tseqno) *Mgr {
	sc := &Mgr{}
	sc.sessions = make(map[string]*SessClnt)
	sc.sid = session
	sc.seqno = seqno
	return sc
}

func (sc *Mgr) SessClnts() []*SessClnt {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	ss := make([]*SessClnt, 0, len(sc.sessions))
	for _, sess := range sc.sessions {
		ss = append(ss, sess)
	}
	return ss
}

// Return an existing sess if there is one, else allocate a new one. Caller
// holds lock.
func (sc *Mgr) allocSessClnt(addrs []string) (*SessClnt, *np.Err) {
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

func (sc *Mgr) RPC(addr []string, req np.Tmsg, f np.Tfence) (np.Tmsg, *np.Err) {
	db.DPrintf("SESSCLNT", "%v RPC %v %v to %v\n", sc.sid, req.Type(), req, addr)
	// Get or establish sessection
	sess, err := sc.allocSessClnt(addr)
	if err != nil {
		db.DPrintf("SESSCLNT", "Unable to alloc sess for req %v %v err %v to %v\n", req.Type(), req, err, addr)
		return nil, err
	}
	msg, err := sess.rpc(req, f)
	if srvClosedSess(msg, err) {
		db.DPrintf("SESSCLNT", "Srv closed sess %v on req %v %v\n", sc.sid, req.Type(), req)
		sess.close()
	}
	return msg, err
}

// Check if the session needs to be closed because the server killed it.
func srvClosedSess(msg np.Tmsg, err *np.Err) bool {
	rerr, ok := msg.(np.Rerror)
	if ok {
		err := np.String2Err(rerr.Ename)
		if np.IsErrClosed(err) {
			return true
		}
	}
	return false
}

// For testing
func (sc *Mgr) Disconnect(addrs []string) *np.Err {
	db.DPrintf("SESSCLNT", "Disconnect %v %v\n", sc.sid, addrs)
	key := sessKey(addrs)
	sc.mu.Lock()
	sess, ok := sc.sessions[key]
	sc.mu.Unlock()
	if !ok {
		return np.MkErr(np.TErrUnreachable, "disconnect: "+sessKey(addrs))
	}
	sess.close()
	return nil
}

func sessKey(addrs []string) string {
	return strings.Join(addrs, ",")
}
