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
	"ulambda/protsrv"
	"ulambda/repl"
	"ulambda/session"
	"ulambda/stats"
	"ulambda/threadmgr"
)

type Snapshot struct {
	Imap         map[unsafe.Pointer]ObjSnapshot
	Root         unsafe.Pointer
	Sts          []byte
	St           []byte
	Tm           []byte
	Rft          []byte
	Rc           []byte
	restoreCache map[unsafe.Pointer]fs.FsObj
}

func MakeSnapshot() *Snapshot {
	s := &Snapshot{}
	s.Imap = make(map[unsafe.Pointer]ObjSnapshot)
	s.Root = nil
	s.restoreCache = make(map[unsafe.Pointer]fs.FsObj)
	return s
}

func (s *Snapshot) Snapshot(root fs.FsObj, sts *stats.Stats, st *session.SessionTable, tm *threadmgr.ThreadMgrTable, rft *fences.RecentTable, rc *repl.ReplyCache) []byte {
	// Snapshot the FS tree.
	s.Root = s.snapshotFsTree(root)
	b, err := json.Marshal(s)
	if err != nil {
		log.Fatalf("Error marshalling snapshot: %v", err)
	}
	// Snapshot stats.
	s.Sts = sts.Snapshot()
	// Snapshot the session table.
	s.St = st.Snapshot()
	// Snapshot the thread manager table.
	s.Tm = tm.Snapshot()
	// Snapshot the recent fence table.
	s.Rft = rft.Snapshot()
	// Snapshot the reply cache.
	s.Rc = rc.Snapshot()
	return b
}

func (s *Snapshot) snapshotFsTree(o fs.FsObj) unsafe.Pointer {
	var ptr unsafe.Pointer
	var snap ObjSnapshot
	switch o.(type) {
	case *dir.DirImpl:
		d := o.(*dir.DirImpl)
		ptr = unsafe.Pointer(d)
		snap = MakeObjSnapshot(Tdir, d.Snapshot(s.snapshotFsTree))
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

func Restore(rps protsrv.RestoreProtServer, pfn threadmgr.ProcessFn, b []byte) (fs.FsObj, *stats.Stats, *session.SessionTable, *threadmgr.ThreadMgrTable, *fences.RecentTable, *repl.ReplyCache) {
	s := MakeSnapshot()
	err := json.Unmarshal(b, s)
	if err != nil {
		log.Fatalf("FATAL error unmarshal file in snapshot.Restore: %v", err)
	}
	root := s.restoreFsTree(s.Root)
	// Restore stats.
	sts := stats.Restore(s.restoreFsTree, s.Sts)
	// Restore the thread manager table.
	tmt := threadmgr.Restore(pfn, b)
	// Restore the recent fence table.
	rft := fences.RestoreRecentTable(s.Rft)
	// Restore the session table.
	st := session.RestoreTable(nil /* TODO: Get mkps */, nil, nil /* TODO: get fssrv */, rft, tmt, s.St)
	// Restore the reply cache.
	rc := repl.Restore(s.Rc)
	return root, sts, st, tmt, rft, rc
}

func (s *Snapshot) restoreFsTree(ptr unsafe.Pointer) fs.FsObj {
	if obj, ok := s.restoreCache[ptr]; ok {
		return obj
	}
	snap := s.Imap[ptr]
	switch snap.Type {
	case Tdir:
		return dir.Restore(s.restoreFsTree, snap.Data)
	case Tfile:
		return memfs.RestoreFile(s.restoreFsTree, snap.Data)
	case Tsymlink:
		return memfs.RestoreSymlink(s.restoreFsTree, snap.Data)
	default:
		log.Fatalf("FATAL error unknown type in Snapshot.restore: %v", snap.Type)
		return nil
	}
}
