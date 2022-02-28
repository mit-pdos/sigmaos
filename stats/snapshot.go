package stats

import (
	"encoding/json"
	"log"
)

func (stats *Stats) Snapshot() []byte {
	b, err := json.Marshal(stats.sti)
	if err != nil {
		log.Fatalf("FATAL Error snapshot encoding stats: %v", err)
	}
	return b
}
