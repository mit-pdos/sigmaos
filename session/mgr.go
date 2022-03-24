package session

import (
	"time"

	db "ulambda/debug"
	np "ulambda/ninep"
)

const (
	HEARTBEATMS   = 50              // Hearbeat every 50 msec.
	SESSTIMEOUTMS = HEARTBEATMS * 4 // Kill a session after 4 missed heartbeats.
)

type SessionMgr struct {
	st      *SessionTable
	process func(*np.Fcall, chan *np.Fcall)
	done    bool
}

func MakeSessionMgr(st *SessionTable, pfn func(*np.Fcall, chan *np.Fcall)) *SessionMgr {
	sm := &SessionMgr{}
	sm.st = st
	sm.process = pfn
	go sm.run()
	return sm
}

// Find timed-out sessions.
func (sm *SessionMgr) getTimedOutSessions() []np.Tsession {
	// Lock the session table.
	sm.st.Lock()
	defer sm.st.Unlock()
	sids := []np.Tsession{}
	for sid, sess := range sm.st.sessions {
		// Find timed-out sessions which haven't been closed yet.
		if sess.timedOut() && !sess.IsClosed() {
			db.DLPrintf("SESSION", "Sess %v timed out", sid)
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
		sids := sm.getTimedOutSessions()
		for _, sid := range sids {
			detach := np.MakeFcall(np.Tdetach{}, sid, nil, np.NoFence)
			sm.process(detach, nil)
		}
	}
}

func (sm *SessionMgr) Done() bool {
	return sm.done
}

func (sm *SessionMgr) Stop() {
	sm.done = true
}
