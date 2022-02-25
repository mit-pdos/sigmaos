package dir

import (
	"encoding/json"
	"log"

	"ulambda/fs"
	"ulambda/inode"
)

// TODO: are there issues with locking?
type DirSnapshot struct {
	InodeSnap []byte
	Entries   map[string]uintptr
}

func makeDirSnapshot(fn fs.SnapshotF, d *DirImpl) []byte {
	ds := &DirSnapshot{}
	ds.InodeSnap = d.FsObj.(*inode.Inode).Snapshot()
	ds.Entries = make(map[string]uintptr)
	for n, e := range d.entries {
		ds.Entries[n] = fn(e)
	}
	b, err := json.Marshal(ds)
	if err != nil {
		log.Fatalf("FATAL Error snapshot encoding DirImpl: %v", err)
	}
	return b
}

func restore(fn fs.RestoreF, b []byte) fs.FsObj {
	ds := &DirSnapshot{}
	err := json.Unmarshal(b, ds)
	if err != nil {
		log.Fatalf("FATAL error unmarshal file in restoreSymlink: %v", err)
	}
	d := &DirImpl{}
	d.FsObj = inode.RestoreInode(fn, ds.InodeSnap)
	for name, ptr := range ds.Entries {
		d.entries[name] = fn(ptr)
	}
	return d
}
