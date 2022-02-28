package fences

import (
	"encoding/json"
	"log"
)

func (rft *RecentTable) Snapshot() []byte {
	b, err := json.Marshal(rft.fences)
	if err != nil {
		log.Fatalf("FATAL Error snapshot encoding recent fence table: %v", err)
	}
	return b
}

func Restore(b []byte) *RecentTable {
	rft := &RecentTable{}
	err := json.Unmarshal(b, rft.fences)
	if err != nil {
		log.Fatalf("FATAL error unmarshal fences in restore: %v", err)
	}
	return rft
}
