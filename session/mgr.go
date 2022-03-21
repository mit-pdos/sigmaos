package session

import (
	"log"
	"sync"
	"time"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/proc"
)

const (
	HEARTBEATMS   = 50              // Hearbeat every 50 msec.
	SESSTIMEOUTMS = HEARTBEATMS * 4 // Kill a session after 4 missed heartbeats.
)

type SessionMgr struct {
	sync.Mutex
	sessions  map[np.Tsession]time.Time
	sessions1 map[np.Tsession]*Session
	process   func(*np.Fcall, chan *np.Fcall)
	done      bool
}

func MakeSessionMgr(pfn func(*np.Fcall, chan *np.Fcall)) *SessionMgr {
	sm := &SessionMgr{}
	sm.sessions = make(map[np.Tsession]time.Time)
	sm.sessions1 = make(map[np.Tsession]*Session)
	sm.process = pfn
	go sm.run()
	return sm
}

// Register a session in the session manager.
func (sm *SessionMgr) RegisterSession(sid np.Tsession, sess *Session) {
	sm.Lock()
	defer sm.Unlock()
	sm.sessions[sid] = time.Now()
	sm.sessions1[sid] = sess
}

// Delete a session from the sessionmgr
func (sm *SessionMgr) DetachSession(sid np.Tsession) {
	sm.Lock()
	defer sm.Unlock()
	db.DLPrintf("SESSIONMGR", "Processed Detach session %v", sid)
	delete(sm.sessions, sid)
}

// Register heartbeats from sids.
func (sm *SessionMgr) Heartbeats(sids []np.Tsession) {
	sm.Lock()
	defer sm.Unlock()
	for _, sid := range sids {
		if _, ok := sm.sessions[sid]; !ok {
			log.Fatalf("%v FATAL heartbeat for unknown session %v", proc.GetName(), sid)
		}
		db.DLPrintf("SESSIONMGR", "Processed heartbeat session %v", sid)
		sm.sessions[sid] = time.Now()
	}
}

// Find timed-out sessions.
func (sm *SessionMgr) getDetachableSessions() []np.Tsession {
	sm.Lock()
	defer sm.Unlock()
	sids := []np.Tsession{}
	for sid, t := range sm.sessions {
		if !sm.sessions1[sid].Running && time.Now().Sub(t).Milliseconds() > SESSTIMEOUTMS {
			db.DLPrintf("SESSIONMGR", "Timeout session %v", sid)
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
			sm.process(detach, nil)
		}
	}
}

func (sm *SessionMgr) Done() bool {
	sm.Lock()
	defer sm.Unlock()
	return sm.done
}
