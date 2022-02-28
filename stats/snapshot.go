package stats

import (
	"encoding/json"
	"log"
	"reflect"
	"unsafe"

	"ulambda/fs"
)

type StatsSnapshot struct {
	Obj  unsafe.Pointer
	Info *StatInfo
}

func MakeStatsSnapshot() *StatsSnapshot {
	return &StatsSnapshot{}
}

func (stats *Stats) Snapshot() []byte {
	ss := MakeStatsSnapshot()
	ss.Obj = unsafe.Pointer(reflect.ValueOf(stats.FsObj).Pointer())
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
	stats.FsObj = fn(ss.Obj)
	stats.sti = ss.Info
	return stats
}
