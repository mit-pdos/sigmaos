package snapshot

import (
	"encoding/json"
	"log"
	"reflect"

	"ulambda/dir"
	"ulambda/fencefs"
	"ulambda/fs"
	"ulambda/inode"
	"ulambda/memfs"
	np "ulambda/ninep"
	"ulambda/overlay"
	"ulambda/protsrv"
	"ulambda/repl"
	"ulambda/session"
	"ulambda/stats"
	"ulambda/threadmgr"
)

type Snapshot struct {
	fssrv        protsrv.FsServer
	Imap         map[np.Tpath]ObjSnapshot
	DirOverlay   []byte
	St           []byte
	Tmt          []byte
	Rc           []byte
	NextInum     uint64
	restoreCache map[np.Tpath]fs.Inode
}

func MakeSnapshot(fssrv protsrv.FsServer) *Snapshot {
	s := &Snapshot{}
	s.fssrv = fssrv
	s.Imap = make(map[np.Tpath]ObjSnapshot)
	s.restoreCache = make(map[np.Tpath]fs.Inode)
	return s
}

func (s *Snapshot) Snapshot(root *overlay.DirOverlay, st *session.SessionTable, tm *threadmgr.ThreadMgrTable, rc *repl.ReplyCache) []byte {
	// Snapshot the FS tree.
	s.DirOverlay = root.Snapshot(s.snapshotFsTree) //s.snapshotFsTree(root)
	// Snapshot the session table.
	s.St = st.Snapshot()
	// Snapshot the thread manager table.
	s.Tmt = tm.Snapshot()
	// Snapshot the reply cache.
	s.Rc = rc.Snapshot()
	b, err := json.Marshal(s)
	if err != nil {
		log.Fatalf("Error marshalling snapshot: %v", err)
	}
	// Store the next inum
	s.NextInum = inode.NextInum
	return b
}

func (s *Snapshot) snapshotFsTree(i fs.Inode) np.Tpath {
	var stype Tsnapshot
	switch i.(type) {
	case *dir.DirImpl:
		stype = Tdir
	case *memfs.File:
		stype = Tfile
	case *memfs.Symlink:
		stype = Tsymlink
	case *fencefs.Fence:
		stype = Tfence
	case *stats.Stats:
		stype = Tstats
	case *Dev:
		stype = Tsnapshotdev
	default:
		log.Fatalf("Unknown FsObj type in snapshot.snapshotFsTree: %v", reflect.TypeOf(i))
	}
	s.Imap[i.Qid().Path] = MakeObjSnapshot(stype, i.Snapshot(s.snapshotFsTree))
	return i.Qid().Path
}

func (s *Snapshot) Restore(mkps protsrv.MkProtServer, rps protsrv.RestoreProtServer, fssrv protsrv.FsServer, tm *threadmgr.ThreadMgr, pfn threadmgr.ProcessFn, oldRc *repl.ReplyCache, b []byte) (fs.FsObj, *session.SessionTable, *threadmgr.ThreadMgrTable, *repl.ReplyCache) {
	err := json.Unmarshal(b, s)
	if err != nil {
		log.Fatalf("FATAL error unmarshal file in snapshot.Restore: %v", err)
	}
	s.restoreCache[0] = nil
	// Restore the next inum
	inode.NextInum = s.NextInum
	// Restore the fs tree
	// TODO: restore dir overlay
	//	root := s.RestoreFsTree(s.Root)
	// Restore the thread manager table and any in-flight ops.
	tmt := threadmgr.Restore(pfn, tm, s.Tmt)
	// Restore the session table.
	st := session.RestoreTable(mkps, rps, fssrv, tmt, s.St)
	// Restore the reply cache.
	rc := repl.Restore(s.Rc)
	// Merge with the current replyCache, because some ops may have arrived &
	// begun executing since this snapshot was taken, and they expect some state
	// to be in the reply cache.
	rc.Merge(oldRc)
	//	return root, st, tmt, rc
	return nil, st, tmt, rc
}

func (s *Snapshot) RestoreFsTree(inum np.Tpath) fs.Inode {
	if obj, ok := s.restoreCache[inum]; ok {
		return obj
	}
	snap := s.Imap[inum]
	var i fs.Inode
	switch snap.Type {
	case Tdir:
		// Make a dir with a nil inode so we don't recurse infinitely when trying
		// to set parent pointers.
		// XXX hard coded memfs.MakeInode
		d := dir.MakeDir(nil, memfs.MakeInode)
		s.restoreCache[inum] = d
		i = dir.Restore(d, s.RestoreFsTree, snap.Data)
	case Tfile:
		i = memfs.RestoreFile(s.RestoreFsTree, snap.Data)
	case Tsymlink:
		i = memfs.RestoreSymlink(s.RestoreFsTree, snap.Data)
	case Tstats:
		i = stats.Restore(s.RestoreFsTree, snap.Data)
	case Tsnapshotdev:
		// TODO: handle snapshot dev and stats dev correctly
		// i = MakeDev(s.fssrv, nil, s.RestoreFsTree(s.Root).(fs.Dir))
	default:
		log.Fatalf("FATAL error unknown type in Snapshot.restore: %v", snap.Type)
		i = nil
	}
	// Store the object in the restore cache.
	s.restoreCache[inum] = i
	return i
}
