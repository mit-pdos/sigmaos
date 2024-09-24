package sessclnt

import (
	"sync"
	//	"github.com/sasha-s/go-deadlock"

	db "sigmaos/debug"
	"sigmaos/netproxyclnt"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

type Mgr struct {
	mu       sync.Mutex
	sessKeys map[*sp.Tendpoint]string
	sessions map[string]*SessClnt
	pe       *proc.ProcEnv
	npc      *netproxyclnt.NetProxyClnt
}

func NewMgr(pe *proc.ProcEnv, npc *netproxyclnt.NetProxyClnt) *Mgr {
	sc := &Mgr{
		sessions: make(map[string]*SessClnt),
		sessKeys: make(map[*sp.Tendpoint]string),
		pe:       pe,
		npc:      npc,
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
func (sc *Mgr) allocSessClnt(ep *sp.Tendpoint) (*SessClnt, *serr.Err) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	key := sc.getSessKeyL(ep)
	if sess, ok := sc.sessions[key]; ok {
		return sess, nil
	}
	sess, err := newSessClnt(sc.pe, sc.npc, ep)
	if err != nil {
		return nil, err
	}
	sc.sessions[key] = sess
	return sess, nil
}

func (sc *Mgr) LookupSessClnt(ep *sp.Tendpoint) (*SessClnt, *serr.Err) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	key := sc.getSessKeyL(ep)
	if sess, ok := sc.sessions[key]; ok {
		return sess, nil
	}
	return nil, serr.NewErr(serr.TErrNotfound, ep)
}

func (sc *Mgr) RPC(ep *sp.Tendpoint, req sessp.Tmsg, iniov sessp.IoVec, outiov sessp.IoVec) (*sessp.FcallMsg, *serr.Err) {
	// Get or establish sessection
	sess, err := sc.allocSessClnt(ep)
	if err != nil {
		db.DPrintf(db.SESSCLNT, "Unable to alloc sess for req %v %v err %v to %v", req.Type(), req, err, ep)
		return nil, err
	}
	rep, err := sess.RPC(req, iniov, outiov)
	return rep, err
}

func (sc *Mgr) getSessKeyL(ep *sp.Tendpoint) string {
	if s, ok := sc.sessKeys[ep]; ok {
		return s
	}
	s := epToSessKey(ep)
	sc.sessKeys[ep] = s
	return s
}

func epToSessKey(ep *sp.Tendpoint) string {
	return ep.Addrs().String()
}
