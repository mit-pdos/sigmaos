package perf

import (
	"time"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

// Some convenience functions for logging performance-related data
func LogSpawnLatency(msg string, pid sp.Tpid, spawnTime time.Time, opStart time.Time) {
	db.DPrintf(db.SPAWN_LAT, "[%s] %s op:%v sinceSpawn:%v", pid, msg, time.Since(opStart), time.Since(spawnTime))
}
