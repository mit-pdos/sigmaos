package snapshot

import (
	"encoding/json"
	"log"
	"reflect"

	"ulambda/dir"
	"ulambda/fences"
	"ulambda/fs"
	"ulambda/inode"
	"ulambda/memfs"
	np "ulambda/ninep"
	"ulambda/protsrv"
	"ulambda/repl"
	"ulambda/session"
	"ulambda/stats"
	"ulambda/threadmgr"
)

type Snapshot struct {
	fssrv        protsrv.FsServer
	Imap         map[np.Tpath]ObjSnapshot
	Root         np.Tpath
	St           []byte
	Tmt          []byte
	Rft          []byte
	Rc           []byte
	NextInum     uint64
	restoreCache map[np.Tpath]fs.Inode
}

func MakeSnapshot(fssrv protsrv.FsServer) *Snapshot {
	s := &Snapshot{}
	s.fssrv = fssrv
	s.Imap = make(map[np.Tpath]ObjSnapshot)
	s.Root = np.Tpath(0)
	s.restoreCache = make(map[np.Tpath]fs.Inode)
	return s
}

func (s *Snapshot) Snapshot(root fs.Inode, st *session.SessionTable, tm *threadmgr.ThreadMgrTable, rft *fences.RecentTable, rc *repl.ReplyCache) []byte {
	// Snapshot the FS tree.
	s.Root = s.snapshotFsTree(root)
	// Snapshot the session table.
	s.St = st.Snapshot()
	// Snapshot the thread manager table.
	s.Tmt = tm.Snapshot()
	// Snapshot the recent fence table.
	s.Rft = rft.Snapshot()
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

func (s *Snapshot) Restore(mkps protsrv.MkProtServer, rps protsrv.RestoreProtServer, fssrv protsrv.FsServer, tm *threadmgr.ThreadMgr, pfn threadmgr.ProcessFn, oldRc *repl.ReplyCache, b []byte) (fs.FsObj, *session.SessionTable, *threadmgr.ThreadMgrTable, *fences.RecentTable, *repl.ReplyCache) {
	err := json.Unmarshal(b, s)
	if err != nil {
		log.Fatalf("FATAL error unmarshal file in snapshot.Restore: %v", err)
	}
	s.restoreCache[0] = nil
	// Restore the next inum
	inode.NextInum = s.NextInum
	// Restore the fs tree
	root := s.RestoreFsTree(s.Root)
	// Restore the thread manager table and any in-flight ops.
	tmt := threadmgr.Restore(pfn, tm, s.Tmt)
	// Restore the recent fence table.
	rft := fences.RestoreRecentTable(s.Rft)
	// Restore the session table.
	st := session.RestoreTable(mkps, rps, fssrv, rft, tmt, s.St)
	// Restore the reply cache.
	rc := repl.Restore(s.Rc)
	// Merge with the current replyCache, because some ops may have arrived &
	// begun executing since this snapshot was taken, and they expect some state
	// to be in the reply cache.
	rc.Merge(oldRc)
	return root, st, tmt, rft, rc
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
		i = MakeDev(s.fssrv, nil, s.RestoreFsTree(s.Root).(fs.Dir))
	default:
		log.Fatalf("FATAL error unknown type in Snapshot.restore: %v", snap.Type)
		i = nil
	}
	// Store the object in the restore cache.
	s.restoreCache[inum] = i
	return i
}
