package repl

import (
	"encoding/json"
	"log"
)

func (rc *ReplyCache) Snapshot() []byte {
	rfs := rc.entries
	// TODO: turn e into a sorted list of fcalls.
	b, err := json.Marshal(rfs)
	if err != nil {
		log.Fatalf("FATAL Error snapshot encoding reply cache: %v", err)
	}
	return b
}
