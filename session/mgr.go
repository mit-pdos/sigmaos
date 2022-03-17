package session

import (
	"log"
	"sync"
	"time"

	np "ulambda/ninep"
	"ulambda/threadmgr"
)

const (
	HEARTBEATMS   = 50              // Hearbeat every 50 msec.
	SESSTIMEOUTMS = HEARTBEATMS * 4 // Kill a session after 4 missed heartbeats.
)

type SessionMgr struct {
	sync.Mutex
	sessions   map[np.Tsession]time.Time
	replyChans map[np.Tsession]chan *np.Fcall
	pfn        threadmgr.ProcessFn
	done       bool
}

func MakeSessionMgr(pfn threadmgr.ProcessFn) *SessionMgr {
	sm := &SessionMgr{}
	sm.sessions = make(map[np.Tsession]time.Time)
	sm.pfn = pfn
	go sm.run()
	return sm
}

// Register a session in the session manager.
func (sm *SessionMgr) RegisterSession(sid np.Tsession) {
	sm.Lock()
	defer sm.Unlock()
	if _, ok := sm.sessions[sid]; ok {
		log.Fatalf("FATAL sessionmgr tried to re-register session %v", sid)
	}
	sm.sessions[sid] = time.Now()
}

// Delete a session from the sessionmgr
func (sm *SessionMgr) DetachSession(sid np.Tsession) {
	sm.Lock()
	defer sm.Unlock()
	delete(sm.sessions, sid)
}

// Register heartbeats from sids.
func (sm *SessionMgr) Heartbeats(sids []np.Tsession) {
	sm.Lock()
	defer sm.Unlock()
	for _, sid := range sids {
		if _, ok := sm.sessions[sid]; !ok {
			log.Fatalf("FATAL heartbeat for unknown session %v", sid)
		}
		sm.sessions[sid] = time.Now()
	}
}

// Find timed-out sessions.
func (sm *SessionMgr) getDetachableSessions() []np.Tsession {
	sm.Lock()
	defer sm.Unlock()
	sids := []np.Tsession{}
	for sid, t := range sm.sessions {
		if time.Now().Sub(t).Milliseconds() > SESSTIMEOUTMS {
			sids = append(sids, sid)
		}
	}
	return sids
}

// Scan for detachable sessions, and request that they be detahed.
func (sm *SessionMgr) run() {
	for !sm.Done() {
		// Sleep for a bit.
		time.Sleep(SESSTIMEOUTMS * time.Millisecond)
		sids := sm.getDetachableSessions()
		for _, sid := range sids {
			detach := np.MakeFcall(np.Tdetach{}, sid, nil)
			sm.pfn(detach, nil)
		}
	}
}

func (sm *SessionMgr) Done() bool {
	sm.Lock()
	defer sm.Unlock()
	return sm.done
}
