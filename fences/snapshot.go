package fences

import (
	"encoding/json"
	"log"

	np "ulambda/ninep"
)

func (rft *RecentTable) Snapshot() []byte {
	b, err := json.Marshal(rft.fences)
	if err != nil {
		log.Fatalf("FATAL Error snapshot encoding recent fence table: %v", err)
	}
	return b
}

func RestoreRecentTable(b []byte) *RecentTable {
	rft := &RecentTable{}
	err := json.Unmarshal(b, rft.fences)
	if err != nil {
		log.Fatalf("FATAL error unmarshal fences in restore: %v", err)
	}
	return rft
}

type EntrySnapshot struct {
	Fences map[np.Tfenceid]np.Tfence
}

func MakeEntrySnapshot() *EntrySnapshot {
	es := &EntrySnapshot{}
	es.Fences = make(map[np.Tfenceid]np.Tfence)
	return es
}

func (e *Entry) Snapshot() []byte {
	es := MakeEntrySnapshot()
	for id, fence := range e.fences {
		es.Fences[id] = fence
	}
	b, err := json.Marshal(es)
	if err != nil {
		log.Fatalf("FATAL Error snapshot encoding fence table entry snapshot: %v", err)
	}
	return b
}

func RestoreEntry(b []byte) *Entry {
	es := MakeEntrySnapshot()
	err := json.Unmarshal(b, es)
	if err != nil {
		log.Fatalf("FATAL error unmarshal fence table entry in restore: %v", err)
	}
	e := makeEntry()
	for id, fence := range es.Fences {
		e.fences[id] = fence
	}
	return e
}

func (ft *FenceTable) Snapshot() []byte {
	fencedDirs := make(map[string][]byte)
	for d, e := range ft.fencedDirs {
		fencedDirs[d] = e.Snapshot()
	}
	b, err := json.Marshal(fencedDirs)
	if err != nil {
		log.Fatalf("FATAL Error snapshot encoding recent fence table: %v", err)
	}
	return b
}

func RestoreFenceTable(b []byte) *FenceTable {
	fencedDirs := make(map[string][]byte)
	err := json.Unmarshal(b, fencedDirs)
	if err != nil {
		log.Fatalf("FATAL error unmarshal fencedDirs in restore: %v", err)
	}
	ft := MakeFenceTable()
	for d, e := range fencedDirs {
		ft.fencedDirs[d] = RestoreEntry(e)
	}
	return ft
}
