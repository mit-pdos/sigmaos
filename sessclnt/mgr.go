package sessclnt

import (
	"sync"
	//	"github.com/sasha-s/go-deadlock"

	db "sigmaos/debug"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type Mgr struct {
	mu       sync.Mutex
	sessions map[string]*SessClnt
	clntnet  string
}

func NewMgr(clntnet string) *Mgr {
	sc := &Mgr{
		sessions: make(map[string]*SessClnt),
		clntnet:  clntnet,
	}
	db.DPrintf(db.SESSCLNT, "Session Mgr for session")
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
func (sc *Mgr) allocSessClnt(addrs sp.Taddrs) (*SessClnt, *serr.Err) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	// Store as concatenation of addresses
	if len(addrs) == 0 {
		return nil, serr.NewErr(serr.TErrInval, addrs)
	}
	key := sessKey(addrs)
	if sess, ok := sc.sessions[key]; ok {
		return sess, nil
	}
	sess, err := newSessClnt(sc.clntnet, addrs)
	if err != nil {
		return nil, err
	}
	sc.sessions[key] = sess
	return sess, nil
}

func (sc *Mgr) LookupSessClnt(addrs sp.Taddrs) (*SessClnt, *serr.Err) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	key := sessKey(addrs)
	if sess, ok := sc.sessions[key]; ok {
		return sess, nil
	}
	return nil, serr.NewErr(serr.TErrNotfound, addrs)
}

func (sc *Mgr) RPC(addr sp.Taddrs, req sessp.Tmsg, data []byte) (*sessp.FcallMsg, *serr.Err) {
	// Get or establish sessection
	sess, err := sc.allocSessClnt(addr)
	if err != nil {
		db.DPrintf(db.SESSCLNT, "Unable to alloc sess for req %v %v err %v to %v", req.Type(), req, err, addr)
		return nil, err
	}
	db.DPrintf(db.SESSCLNT, "sess %v RPC %v %v to %v", sess.sid, req.Type(), req, addr)
	msg, err := sess.RPC(req, data)
	return msg, err
}

func sessKey(addrs sp.Taddrs) string {
	return addrs.String()
}
