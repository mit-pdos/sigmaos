package threadmgr

import (
	"encoding/json"
	"log"
	"sort"

	np "ulambda/ninep"
	"ulambda/npcodec"
)

type OpSnapshot struct {
	Fc []byte
	N  uint64
}

func MakeOpSnapshot(fc *np.Fcall, n uint64) *OpSnapshot {
	b, err := npcodec.Marshal(fc)
	if err != nil {
		log.Fatalf("FATAL error marshalling fcall in MakeOpSnapshot: %v", err)
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
		log.Fatalf("FATAL Error snapshot encoding thread manager table: %v", err)
	}
	return b
}

func Restore(pfn ProcessFn, b []byte) *ThreadMgrTable {
	tmt := MakeThreadMgrTable(pfn, true)
	// Make a thread (there will only ever be one since we're running replicated)
	tm := tmt.AddThread()
	opss := []*OpSnapshot{}
	err := json.Unmarshal(b, &opss)
	if err != nil {
		log.Fatalf("FATAL error unmarshal threadmgr in restore: %v, \n%v", err, string(b))
	}
	for _, op := range opss {
		fc := &np.Fcall{}
		err1 := npcodec.Unmarshal(op.Fc, fc)
		if err1 != nil {
			log.Fatalf("FATAL error unmarshal fcall in ThreadMgrTable.Restore: %v")
		}
		tm.executing[makeOp(fc, nil, op.N)] = true
	}
	return tmt
}
