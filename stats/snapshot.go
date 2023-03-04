package stats

import (
	"encoding/json"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/inode"
)

type StatsSnapshot struct {
	InodeSnap []byte
	Stats     *Stats
}

func MakeStatsSnapshot() *StatsSnapshot {
	return &StatsSnapshot{}
}

func (stats *StatInfo) snapshot() []byte {
	ss := MakeStatsSnapshot()
	ss.InodeSnap = stats.Inode.Snapshot(nil)
	ss.Stats = stats.st
	b, err := json.Marshal(ss)
	if err != nil {
		db.DFatalf("Error snapshot encoding stats: %v", err)
	}
	return b
}

func Restore(fn fs.RestoreF, b []byte) *StatInfo {
	ss := MakeStatsSnapshot()
	err := json.Unmarshal(b, ss)
	if err != nil {
		db.DFatalf("error unmarshal stats in restore: %v", err)
	}
	stats := &StatInfo{}
	stats.Inode = inode.RestoreInode(fn, ss.InodeSnap)
	stats.st = ss.Stats
	return stats
}
