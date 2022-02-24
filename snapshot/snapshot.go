package snapshot

import (
	"encoding/json"
	"log"
	"reflect"
	"unsafe"

	"ulambda/fs"
	"ulambda/memfs"
)

type Snapshot struct {
	Imap map[uintptr][]byte
	Root uintptr
}

func MakeSnapshot() *Snapshot {
	s := &Snapshot{}
	s.Imap = make(map[uintptr][]byte)
	s.Root = 0
	return s
}

func (s *Snapshot) Serialize(root fs.FsObj) []byte {
	s.Root = s.serialize(root)
	b, err := json.Marshal(s)
	if err != nil {
		log.Fatalf("Error marshalling snapshot: %v", err)
	}
	return b
}

func (s *Snapshot) serialize(o fs.FsObj) uintptr {
	var ptr uintptr
	var b []byte
	switch o.(type) {
	case *memfs.File:
		f := o.(*memfs.File)
		ptr = uintptr(unsafe.Pointer(f))
	default:
		log.Fatalf("Unknown FsObj type in serde.Snapshot.serialize: %v", reflect.TypeOf(o))
	}
	// TODO: get byte representation
	s.Imap[ptr] = b
	return ptr
}
