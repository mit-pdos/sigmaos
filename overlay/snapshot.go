package overlay

import (
	"encoding/json"
	"log"
	"runtime/debug"

	"ulambda/dir"
	"ulambda/fs"
	"ulambda/inode"
	np "ulambda/ninep"
)

type DirOverlaySnapshot struct {
	Root      np.Tpath
	InodeSnap []byte
	Entries   map[string]np.Tpath
}

func makeDirOverlaySnapshot(fn fs.SnapshotF, d *DirOverlay) []byte {
	ds := &DirOverlaySnapshot{}
	// Snapshot the underlying fs tree.
	ds.Root = fn(d.underlay.(*dir.DirImpl))
	ds.InodeSnap = d.Inode.Snapshot(fn)
	ds.Entries = make(map[string]np.Tpath)
	for e, obj := range d.entries {
		if e != np.STATSD && e != np.FENCEDIR && e != np.SNAPDEV {
			log.Fatalf("Unknown mount type in overlay dir: %v", e)
		}
		// Snapshot underlying entries
		ds.Entries[e] = fn(obj)
	}
	return encode(ds)
}

func encode(o interface{}) []byte {
	b, err := json.Marshal(o)
	if err != nil {
		debug.PrintStack()
		log.Fatalf("FATAL Error snapshot encoding diroverlay: %v", err)
	}
	return b
}

func restoreDirOverlay(d *DirOverlay, fn fs.RestoreF, b []byte) fs.Inode {
	ds := &DirOverlaySnapshot{}
	err := json.Unmarshal(b, ds)
	if err != nil {
		log.Fatalf("FATAL error unmarshal diroverlay in restoreDirOverlay (snaplen:%v): %v", len(b), err)
	}
	d.Inode = inode.RestoreInode(fn, ds.InodeSnap)
	root := fn(ds.Root)
	d.underlay = root.(fs.Dir)
	for e, s := range ds.Entries {
		d.entries[e] = fn(s)
	}
	return d
}
