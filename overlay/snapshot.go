package overlay

import (
	"encoding/json"
	"log"
	"runtime/debug"

	"ulambda/dir"
	"ulambda/fs"
	np "ulambda/ninep"
)

type DirOverlaySnapshot struct {
	Root    np.Tpath
	Entries map[string]np.Tpath
}

func makeDirOverlaySnapshot(fn fs.SnapshotF, d *DirOverlay) []byte {
	ds := &DirOverlaySnapshot{}
	// Snapshot the underlying fs tree.
	ds.Root = fn(d.Dir.(*dir.DirImpl))
	ds.Entries = make(map[string]np.Tpath)
	for e, obj := range d.entries {
		if e != np.STATSD && e != np.FENCEDIR && e != np.SNAPDEV {
			log.Fatalf("Unknown mount type in overlay dir: %v", e)
		}
		log.Printf("Overlay dir snapshot %v", e)
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
