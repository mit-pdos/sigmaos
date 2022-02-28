package threadmgr

import (
	"encoding/json"
	"log"
)

func (tmt *ThreadMgrTable) Snapshot() []byte {
	// Since this only happens when replicated, we expect there to only be one
	// threadmgr, wich AddThread should return.
	tm := tmt.AddThread()
	e := tm.GetExecuting()
	// TODO: turn e into a sorted list of fcalls.
	b, err := json.Marshal(e)
	if err != nil {
		log.Fatalf("FATAL Error snapshot encoding thread manager table: %v", err)
	}
	return b
}
