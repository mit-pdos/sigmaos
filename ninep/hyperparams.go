package ninep

// Session constants
const (
	SESSHEARTBEATMS = 2000                 // Client heartbeat frequency.
	SESSTIMEOUTMS   = SESSHEARTBEATMS * 40 // Kill a session after 4 missed heartbeats.
)

// Realm constants
const (
	REALM_SCAN_INTERVAL_MS          = 50  // Frequency with which realmmgr scans to rebalance realms.
	REALM_RESIZE_INTERVAL_MS        = 100 // Maximum frequency with which realmmgr resizes a realm.
	REALM_GROW_CPU_UTIL_THRESHOLD   = 50  // Utilization threshold at which to grow a realm.
	REALM_SHRINK_CPU_UTIL_THRESHOLD = 25  // Utilization threshold at which to shrink a realm.
)

// Procd constants
const (
	PROCD_WORK_STEAL_TIMEOUT_MS = 100 // Procd work steal frequency.
)

// Raft constants
const (
	RAFT_TICK_MS         = 25 // Frequency with which the raft library ticks
	RAFT_ELECT_NTICKS    = 4  // Number of ticks with no leader heartbeat after which a follower starts an election.
	RAFT_HEARTBEAT_TICKS = 1  // Number of heartbeats per tick.
)
