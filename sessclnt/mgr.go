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
	cli      sessp.Tclient
	sessions map[string]*SessClnt
	clntnet  string
}

func MakeMgr(cli sessp.Tclient, clntnet string) *Mgr {
	sc := &Mgr{}
	sc.cli = cli
	sc.sessions = make(map[string]*SessClnt)
	sc.clntnet = clntnet
	db.DPrintf(db.SESS_STATE_CLNT, "Session Mgr for client %v", sc.cli)
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
	key := sessKey(addrs)
	if sess, ok := sc.sessions[key]; ok {
		return sess, nil
	}
	sess, err := makeSessClnt(sc.cli, sc.clntnet, addrs)
	if err != nil {
		return nil, err
	}
	sc.sessions[key] = sess
	return sess, nil
}

func (sc *Mgr) RPC(addr sp.Taddrs, req sessp.Tmsg, data []byte, f *sessp.Tfence) (*sessp.FcallMsg, *serr.Err) {
	// Get or establish sessection
	sess, err := sc.allocSessClnt(addr)
	if err != nil {
		db.DPrintf(db.SESS_STATE_CLNT, "Unable to alloc sess for req %v %v err %v to %v", req.Type(), req, err, addr)
		return nil, err
	}
	db.DPrintf(db.SESS_STATE_CLNT, "cli %v sess %v RPC %v %v to %v", sc.cli, sess.sid, req.Type(), req, addr)
	msg, err := sess.RPC(req, data, f)
	return msg, err
}

// For testing
func (sc *Mgr) Disconnect(addrs sp.Taddrs) *serr.Err {
	db.DPrintf(db.SESS_STATE_CLNT, "Disconnect cli %v addr %v", sc.cli, addrs)
	key := sessKey(addrs)
	sc.mu.Lock()
	sess, ok := sc.sessions[key]
	sc.mu.Unlock()
	if !ok {
		return serr.MkErr(serr.TErrUnreachable, "disconnect: "+sessKey(addrs))
	}
	sess.close()
	db.DPrintf(db.SESS_STATE_CLNT, "Disconnected cli %v sid %v addr %v", sc.cli, sess.sid, addrs)
	return nil
}

func sessKey(addrs sp.Taddrs) string {
	return addrs.Taddrs2String()
}
