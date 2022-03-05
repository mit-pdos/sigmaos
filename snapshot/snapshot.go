package snapshot

import (
	"encoding/json"
	"log"
	"reflect"

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
	Imap         map[uint64]ObjSnapshot
	Root         uint64
	St           []byte
	Tm           []byte
	Rft          []byte
	Rc           []byte
	restoreCache map[uint64]fs.FsObj
}

func MakeSnapshot() *Snapshot {
	s := &Snapshot{}
	s.Imap = make(map[uint64]ObjSnapshot)
	s.Root = 0
	s.restoreCache = make(map[uint64]fs.FsObj)
	return s
}

func (s *Snapshot) Snapshot(root fs.FsObj, st *session.SessionTable, tm *threadmgr.ThreadMgrTable, rft *fences.RecentTable, rc *repl.ReplyCache) []byte {
	// Snapshot the FS tree.
	s.Root = s.snapshotFsTree(root)
	b, err := json.Marshal(s)
	if err != nil {
		log.Fatalf("Error marshalling snapshot: %v", err)
	}
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

func (s *Snapshot) snapshotFsTree(o fs.FsObj) uint64 {
	var stype Tsnapshot
	switch o.(type) {
	case *dir.DirImpl:
		stype = Tdir
	case *memfs.File:
		stype = Tfile
	case *memfs.Symlink:
		stype = Tsymlink
	case *stats.Stats:
		stype = Tstats
	case *Dev:
		stype = Tsnapshotdev
	default:
		log.Fatalf("Unknown FsObj type in snapshot.snapshotFsTree: %v", reflect.TypeOf(o))
	}
	s.Imap[o.Inum()] = MakeObjSnapshot(stype, o.Snapshot(s.snapshotFsTree))
	return o.Inum()
}

func Restore(mkps protsrv.MkProtServer, rps protsrv.RestoreProtServer, fssrv protsrv.FsServer, pfn threadmgr.ProcessFn, b []byte) (fs.FsObj, *session.SessionTable, *threadmgr.ThreadMgrTable, *fences.RecentTable, *repl.ReplyCache) {
	s := MakeSnapshot()
	err := json.Unmarshal(b, s)
	if err != nil {
		log.Fatalf("FATAL error unmarshal file in snapshot.Restore: %v", err)
	}
	s.restoreCache[0] = nil
	root := s.restoreFsTree(s.Root)
	// Restore the thread manager table.
	tmt := threadmgr.Restore(pfn, b)
	// Restore the recent fence table.
	rft := fences.RestoreRecentTable(s.Rft)
	// Restore the session table.
	st := session.RestoreTable(mkps, rps, fssrv, rft, tmt, s.St)
	// Restore the reply cache.
	rc := repl.Restore(s.Rc)
	return root, st, tmt, rft, rc
}

func (s *Snapshot) restoreFsTree(inum uint64) fs.FsObj {
	if obj, ok := s.restoreCache[inum]; ok {
		return obj
	}
	snap := s.Imap[inum]
	switch snap.Type {
	case Tdir:
		return dir.Restore(s.restoreFsTree, snap.Data)
	case Tfile:
		return memfs.RestoreFile(s.restoreFsTree, snap.Data)
	case Tsymlink:
		return memfs.RestoreSymlink(s.restoreFsTree, snap.Data)
	case Tstats:
		return stats.Restore(s.restoreFsTree, snap.Data)
	default:
		log.Fatalf("FATAL error unknown type in Snapshot.restore: %v", snap.Type)
		return nil
	}
}
