package dir

import (
	"encoding/json"
	"runtime/debug"

	"github.com/umpc/go-sortedmap"

	db "ulambda/debug"
	"ulambda/fs"
	"ulambda/inode"
	np "ulambda/ninep"
)

type DirSnapshot struct {
	InodeSnap []byte
	Entries   map[string]np.Tpath
}

func makeDirSnapshot(fn fs.SnapshotF, d *DirImpl) []byte {
	ds := &DirSnapshot{}
	ds.InodeSnap = d.Inode.Snapshot(fn)
	ds.Entries = make(map[string]np.Tpath)
	d.entries.IterFunc(false, func(rec sortedmap.Record) bool {
		if rec.Key == "." {
			return true
		}
		ds.Entries[rec.Key.(string)] = fn(rec.Val.(fs.Inode))
		return true

	})
	b, err := json.Marshal(ds)
	if err != nil {
		db.DFatalf("Error snapshot encoding DirImpl: %v", err)
	}
	return b
}

func restore(d *DirImpl, fn fs.RestoreF, b []byte) fs.Inode {
	ds := &DirSnapshot{}
	err := json.Unmarshal(b, ds)
	if err != nil {
		debug.PrintStack()
		db.DFatalf("error unmarshal file in restoreDir: %v, %v", err, string(b))
	}
	d.Inode = inode.RestoreInode(fn, ds.InodeSnap)
	for name, ptr := range ds.Entries {
		d.entries.Insert(name, fn(ptr))
	}
	return d
}
