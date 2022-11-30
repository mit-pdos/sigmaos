package sessclnt

import (
	"strings"
	"sync"
	//	"github.com/sasha-s/go-deadlock"

	db "sigmaos/debug"
	"sigmaos/fcall"
	np "sigmaos/sigmap"
)

type Mgr struct {
	mu       sync.Mutex
	cli      fcall.Tclient
	sessions map[string]*SessClnt
}

func MakeMgr(cli fcall.Tclient) *Mgr {
	sc := &Mgr{}
	sc.cli = cli
	sc.sessions = make(map[string]*SessClnt)
	db.DPrintf("SESSCLNT", "Session Mgr for client %v", sc.cli)
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
func (sc *Mgr) allocSessClnt(addrs []string) (*SessClnt, *fcall.Err) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	// Store as concatenation of addresses
	key := sessKey(addrs)
	if sess, ok := sc.sessions[key]; ok {
		return sess, nil
	}
	sess, err := makeSessClnt(sc.cli, addrs)
	if err != nil {
		return nil, err
	}
	sc.sessions[key] = sess
	return sess, nil
}

func (sc *Mgr) RPC(addr []string, req np.Tmsg, f *np.Tfence) (np.Tmsg, *fcall.Err) {
	// Get or establish sessection
	sess, err := sc.allocSessClnt(addr)
	if err != nil {
		db.DPrintf("SESSCLNT", "Unable to alloc sess for req %v %v err %v to %v", req.Type(), req, err, addr)
		return nil, err
	}
	db.DPrintf("SESSCLNT", "cli %v sess %v RPC %v %v to %v", sc.cli, sess.sid, req.Type(), req, addr)
	msg, err := sess.RPC(req, f)
	return msg, err
}

// For testing
func (sc *Mgr) Disconnect(addrs []string) *fcall.Err {
	db.DPrintf("SESSCLNT", "Disconnect cli %v addr %v", sc.cli, addrs)
	key := sessKey(addrs)
	sc.mu.Lock()
	sess, ok := sc.sessions[key]
	sc.mu.Unlock()
	if !ok {
		return fcall.MkErr(fcall.TErrUnreachable, "disconnect: "+sessKey(addrs))
	}
	sess.close()
	db.DPrintf("SESSCLNT", "Disconnected cli %v sid %v addr %v", sc.cli, sess.sid, addrs)
	return nil
}

func sessKey(addrs []string) string {
	return strings.Join(addrs, ",")
}
