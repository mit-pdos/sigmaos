package snapshot

import (
	"encoding/json"
	"log"
	"reflect"
	"unsafe"

	"ulambda/dir"
	"ulambda/fs"
	"ulambda/memfs"
)

type Snapshot struct {
	Imap map[uintptr]ObjSnapshot
	Root uintptr
}

func MakeSnapshot() *Snapshot {
	s := &Snapshot{}
	s.Imap = make(map[uintptr]ObjSnapshot)
	s.Root = 0
	return s
}

func (s *Snapshot) Snapshot(root fs.FsObj) []byte {
	s.Root = s.snapshot(root)
	b, err := json.Marshal(s)
	if err != nil {
		log.Fatalf("Error marshalling snapshot: %v", err)
	}
	return b
}

func (s *Snapshot) snapshot(o fs.FsObj) uintptr {
	var ptr uintptr
	var snap ObjSnapshot
	switch o.(type) {
	case *dir.DirImpl:
		d := o.(*dir.DirImpl)
		ptr = uintptr(unsafe.Pointer(d))
		snap = MakeObjSnapshot(Tdir, d.Snapshot(s.snapshot))
	case *memfs.File:
		f := o.(*memfs.File)
		ptr = uintptr(unsafe.Pointer(f))
		snap = MakeObjSnapshot(Tfile, f.Snapshot())
	case *memfs.Symlink:
		f := o.(*memfs.Symlink)
		ptr = uintptr(unsafe.Pointer(f))
		snap = MakeObjSnapshot(Tsymlink, f.Snapshot())
	default:
		log.Fatalf("Unknown FsObj type in serde.Snapshot.serialize: %v", reflect.TypeOf(o))
	}
	s.Imap[ptr] = snap
	return ptr
}
