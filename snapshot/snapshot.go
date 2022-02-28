package snapshot

import (
	"encoding/json"
	"log"
	"reflect"
	"unsafe"

	"ulambda/dir"
	"ulambda/fences"
	"ulambda/fs"
	"ulambda/memfs"
	"ulambda/repl"
	"ulambda/session"
	"ulambda/threadmgr"
)

type Snapshot struct {
	Imap         map[unsafe.Pointer]ObjSnapshot
	Root         unsafe.Pointer
	Rft          []byte
	restoreCache map[unsafe.Pointer]fs.FsObj
}

func MakeSnapshot() *Snapshot {
	s := &Snapshot{}
	s.Imap = make(map[unsafe.Pointer]ObjSnapshot)
	s.Root = nil
	s.restoreCache = make(map[unsafe.Pointer]fs.FsObj)
	return s
}

func (s *Snapshot) Snapshot(root fs.FsObj, st *session.SessionTable, tm *threadmgr.ThreadMgrTable, rft *fences.RecentTable, rc *repl.ReplyCache) []byte {
	// Snapshot the FS tree.
	s.Root = s.snapshot(root)
	b, err := json.Marshal(s)
	if err != nil {
		log.Fatalf("Error marshalling snapshot: %v", err)
	}
	// TODO: Snapshot the session table.
	// TODO: Snapshot the thread manager table.
	// Snapshot the recent fence table.
	s.Rft = rft.Snapshot()
	// TODO: Snapshot the reply cache.
	return b
}

// XXX Do we ever snapshot the same object twice? I don't think so, because I
// believe the structure is a proper tree?
func (s *Snapshot) snapshot(o fs.FsObj) unsafe.Pointer {
	var ptr unsafe.Pointer
	var snap ObjSnapshot
	switch o.(type) {
	case *dir.DirImpl:
		d := o.(*dir.DirImpl)
		ptr = unsafe.Pointer(d)
		snap = MakeObjSnapshot(Tdir, d.Snapshot(s.snapshot))
	case *memfs.File:
		f := o.(*memfs.File)
		ptr = unsafe.Pointer(f)
		snap = MakeObjSnapshot(Tfile, f.Snapshot())
	case *memfs.Symlink:
		f := o.(*memfs.Symlink)
		ptr = unsafe.Pointer(f)
		snap = MakeObjSnapshot(Tsymlink, f.Snapshot())
	case *memfs.Pipe:
		// TODO: plan for snapshotting pipes.
		log.Fatalf("FATAL Tried to snapshot a pipe.")
	default:
		log.Fatalf("Unknown FsObj type in serde.Snapshot.serialize: %v", reflect.TypeOf(o))
	}
	s.Imap[ptr] = snap
	return ptr
}

func Restore(b []byte) fs.FsObj {
	s := MakeSnapshot()
	err := json.Unmarshal(b, s)
	if err != nil {
		log.Fatalf("FATAL error unmarshal file in snapshot.Restore: %v", err)
	}
	root := s.restore(s.Root)
	return root
}

func (s *Snapshot) restore(ptr unsafe.Pointer) fs.FsObj {
	if obj, ok := s.restoreCache[ptr]; ok {
		return obj
	}
	snap := s.Imap[ptr]
	switch snap.Type {
	case Tdir:
		return dir.Restore(s.restore, snap.Data)
	case Tfile:
		return memfs.RestoreFile(s.restore, snap.Data)
	case Tsymlink:
		return memfs.RestoreSymlink(s.restore, snap.Data)
	default:
		log.Fatalf("FATAL error unknown type in Snapshot.restore: %v", snap.Type)
		return nil
	}
}
