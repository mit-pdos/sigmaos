package ninep

// Session constants
const (
	SESSHEARTBEATMS = 50                  // Client heartbeat frequency.
	SESSTIMEOUTMS   = SESSHEARTBEATMS * 4 // Kill a session after 4 missed heartbeats.
)

// Procd constants
const (
	WORK_STEAL_TIMEOUT_MS = 100 // Procd work steal frequency.
)
