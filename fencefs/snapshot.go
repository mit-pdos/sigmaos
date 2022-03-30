package fencefs

import (
	"encoding/json"
	"runtime/debug"

	db "ulambda/debug"
	"ulambda/fs"
	"ulambda/inode"
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
		db.DFatalf("FATAL Error snapshot encoding fence: %v", err)
	}
	return b
}

func restoreFence(fn fs.RestoreF, b []byte) fs.Inode {
	s := &FenceSnapshot{}
	err := json.Unmarshal(b, s)
	if err != nil {
		db.DFatalf("FATAL error unmarshal fence in restoreFence: %v", err)
	}
	f := &Fence{}
	f.Inode = inode.RestoreInode(fn, s.InodeSnap)
	f.epoch = s.Epoch
	return f
}
