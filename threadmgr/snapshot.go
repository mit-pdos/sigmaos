package threadmgr

import (
	"encoding/json"
	"sort"

	db "sigmaos/debug"
	np "sigmaos/ninep"
	"sigmaos/npcodec"
)

type OpSnapshot struct {
	Fc []byte
	N  uint64
}

func MakeOpSnapshot(fc *np.Fcall, n uint64) *OpSnapshot {
	b, err := npcodec.MarshalFcallByte(fc)
	if err != nil {
		db.DFatalf("error marshalling fcall in MakeOpSnapshot: %v", err)
	}
	return &OpSnapshot{b, n}
}

func (tmt *ThreadMgrTable) snapshot() []byte {
	// Since this only happens when replicated, we expect there to only be one
	// threadmgr, wich AddThread should return.
	tm := tmt.AddThread()
	ops := make([]*Op, len(tm.executing))
	idx := 0
	for op, _ := range tm.executing {
		ops[idx] = op
		idx++
	}
	// Sort op list in order of reception.
	sort.Slice(ops, func(i, j int) bool {
		return ops[i].N < ops[j].N
	})
	opss := make([]*OpSnapshot, len(ops))
	for idx, op := range ops {
		opss[idx] = MakeOpSnapshot(op.Fc, op.N)
	}
	b, err := json.Marshal(opss)
	if err != nil {
		db.DFatalf("Error snapshot encoding thread manager table: %v", err)
	}
	return b
}

func Restore(pfn ProcessFn, tm *ThreadMgr, b []byte) *ThreadMgrTable {
	tmt := MakeThreadMgrTable(pfn, true)
	// Add the existing thread manager for the restoring thread.
	tmt.threadmgrs[tm] = true
	// Make a thread (there will only ever be one since we're running replicated)
	opss := []*OpSnapshot{}
	err := json.Unmarshal(b, &opss)
	if err != nil {
		db.DFatalf("error unmarshal threadmgr in restore: %v, \n%v", err, string(b))
	}
	// List of ops currently executing.
	executing := []*Op{}
	for _, op := range opss {
		fc, err1 := npcodec.UnmarshalFcall(op.Fc)
		if err1 != nil {
			db.DFatalf("error unmarshal fcall in ThreadMgrTable.Restore: %v")
		}
		executing = append(executing, makeOp(fc, op.N))
	}
	// Make sure to chop off the last op (which will be the snapshot op).
	executing = executing[:len(executing)-1]

	// Replay blocked ops to recreate server-side state (e.g. sessconds)
	tm.replayOps(executing)

	return tmt
}
