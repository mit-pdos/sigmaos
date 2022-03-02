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

func Restore(pfn ProcessFn, b []byte) *ThreadMgrTable {
	tmt := MakeThreadMgrTable(pfn, true)
	// Make a thread (there will only ever be one since we're running replicated)
	tm := tmt.AddThread()
	err := json.Unmarshal(b, tm.executing)
	if err != nil {
		log.Fatalf("FATAL error unmarshal threadmgr in restore: %v", err)
	}
	return tmt
}
