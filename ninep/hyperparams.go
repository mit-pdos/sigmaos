package ninep

// Session constants
const (
	SESSHEARTBEATMS = 50                  // Hearbeat every 50 msec.
	SESSTIMEOUTMS   = SESSHEARTBEATMS * 4 // Kill a session after 4 missed heartbeats.
)
