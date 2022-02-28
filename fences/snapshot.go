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
