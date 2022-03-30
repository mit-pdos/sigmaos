package dir

import (
	"encoding/json"
	"log"
	"runtime/debug"

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
	for n, e := range d.entries {
		if n == "." {
			continue
		}
		ds.Entries[n] = fn(e)
	}
	b, err := json.Marshal(ds)
	if err != nil {
		log.Fatalf("FATAL Error snapshot encoding DirImpl: %v", err)
	}
	return b
}

func restore(d *DirImpl, fn fs.RestoreF, b []byte) fs.Inode {
	ds := &DirSnapshot{}
	err := json.Unmarshal(b, ds)
	if err != nil {
		debug.PrintStack()
		log.Fatalf("FATAL error unmarshal file in restoreDir: %v, %v", err, string(b))
	}
	d.Inode = inode.RestoreInode(fn, ds.InodeSnap)
	for name, ptr := range ds.Entries {
		d.entries[name] = fn(ptr)
	}
	return d
}
