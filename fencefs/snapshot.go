package fencefs

import (
	"encoding/json"
	"log"
	"runtime/debug"

	"ulambda/fs"
	np "ulambda/ninep"
)

type FenceSnapshot struct {
	InodeSnap []byte
	Epoch     np.Tepoch
}

func makeFenceSnapshot(fn fs.SnapshotF, f *Fence) []byte {
	s := &FenceSnapshot{}
	s.InodeSnap = f.Inode.Snapshot(nil)
	s.Epoch = f.epoch
	return encode(s)
}

func encode(o interface{}) []byte {
	b, err := json.Marshal(o)
	if err != nil {
		debug.PrintStack()
		log.Fatalf("FATAL Error snapshot encoding fence: %v", err)
	}
	return b
}
