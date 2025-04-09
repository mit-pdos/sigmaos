package perf

import (
	"time"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

var (
	TIME_NOT_SET time.Time = time.Unix(0, 0)
)

// Some convenience functions for logging performance-related data
func LogSpawnLatency(msg string, pid sp.Tpid, spawnTime time.Time, opStart time.Time) {
	// Bail out early if not logging
	if !db.WillBePrinted(db.SPAWN_LAT) {
		return
	}
	var sinceSpawn time.Duration
	if spawnTime != TIME_NOT_SET {
		sinceSpawn = time.Since(spawnTime)
	}
	var sinceOpStart time.Duration
	if opStart != TIME_NOT_SET {
		sinceOpStart = time.Since(opStart)
	}
	db.DPrintf(db.SPAWN_LAT, "[%s] %s op:%v sinceSpawn:%v", pid, msg, sinceOpStart, sinceSpawn)
}
