package stats

import (
	"encoding/json"
	"log"

	"ulambda/fs"
	"ulambda/inode"
)

type StatsSnapshot struct {
	InodeSnap []byte
	Info      *StatInfo
}

func MakeStatsSnapshot() *StatsSnapshot {
	return &StatsSnapshot{}
}

func (stats *Stats) snapshot() []byte {
	ss := MakeStatsSnapshot()
	ss.InodeSnap = stats.FsObj.Snapshot(nil)
	ss.Info = stats.sti
	b, err := json.Marshal(ss)
	if err != nil {
		log.Fatalf("FATAL Error snapshot encoding stats: %v", err)
	}
	return b
}

func Restore(fn fs.RestoreF, b []byte) *Stats {
	ss := MakeStatsSnapshot()
	err := json.Unmarshal(b, ss)
	if err != nil {
		log.Fatalf("FATAL error unmarshal stats in restore: %v", err)
	}
	stats := &Stats{}
	stats.FsObj = inode.RestoreInode(fn, ss.InodeSnap)
	stats.sti = ss.Info
	return stats
}
