package snapshot

import (
	"encoding/json"
	"log"
	"reflect"

	db "ulambda/debug"
	"ulambda/dir"
	"ulambda/fencefs"
	"ulambda/fs"
	"ulambda/inode"
	"ulambda/memfs"
	np "ulambda/ninep"
	"ulambda/overlay"
	"ulambda/sessstatesrv"
	"ulambda/stats"
	"ulambda/threadmgr"
)

type Snapshot struct {
	sesssrv      np.SessServer
	Imap         map[np.Tpath]ObjSnapshot
	DirOverlay   np.Tpath
	St           []byte
	Tmt          []byte
	NextInum     uint64
	restoreCache map[np.Tpath]fs.Inode
}

func MakeSnapshot(sesssrv np.SessServer) *Snapshot {
	s := &Snapshot{}
	s.sesssrv = sesssrv
	s.Imap = make(map[np.Tpath]ObjSnapshot)
	s.restoreCache = make(map[np.Tpath]fs.Inode)
	return s
}

func (s *Snapshot) Snapshot(root *overlay.DirOverlay, st *sessstatesrv.SessionTable, tm *threadmgr.ThreadMgrTable) []byte {
	// Snapshot the FS tree.
	s.DirOverlay = s.snapshotFsTree(root)
	// Snapshot the session table.
	s.St = st.Snapshot()
	// Snapshot the thread manager table.
	s.Tmt = tm.Snapshot()
	b, err := json.Marshal(s)
	if err != nil {
		db.DFatalf("Error marshalling snapshot: %v", err)
	}
	// Store the next inum
	s.NextInum = inode.NextInum
	return b
}

func (s *Snapshot) snapshotFsTree(i fs.Inode) np.Tpath {
	var stype Tsnapshot
	switch i.(type) {
	case *overlay.DirOverlay:
		log.Printf("Snapshot DirOverlay with path %v", i.Path)
		stype = Toverlay
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
		db.DFatalf("Unknown FsObj type in snapshot.snapshotFsTree: %v", reflect.TypeOf(i))
	}
	s.Imap[i.Path()] = MakeObjSnapshot(stype, i.Snapshot(s.snapshotFsTree))
	return i.Path()
}

func (s *Snapshot) Restore(mkps np.MkProtServer, rps np.RestoreProtServer, sesssrv np.SessServer, tm *threadmgr.ThreadMgr, pfn threadmgr.ProcessFn, oldSt *sessstatesrv.SessionTable, b []byte) (fs.Dir, fs.Dir, *stats.Stats, *sessstatesrv.SessionTable, *threadmgr.ThreadMgrTable) {
	err := json.Unmarshal(b, s)
	if err != nil {
		db.DFatalf("error unmarshal file in snapshot.Restore: %v", err)
	}
	s.restoreCache[0] = nil
	// Restore the next inum
	inode.NextInum = s.NextInum
	// Restore the fs tree
	dirover := s.RestoreFsTree(s.DirOverlay).(*overlay.DirOverlay) //overlay.Restore(s.RestoreFsTree, s.DirOverlay)
	// Get the ffs & stats
	_, ffs, _, _ := dirover.Lookup(nil, np.Split(np.FENCEDIR))
	_, stat, _, _ := dirover.Lookup(nil, np.Split(np.STATSD))
	// Fix up the sesssrv pointer in snapshotdev
	_, dev, _, _ := dirover.Lookup(nil, np.Split(np.SNAPDEV))
	dev.(*Dev).srv = sesssrv
	// Restore the thread manager table and any in-flight ops.
	tmt := threadmgr.Restore(pfn, tm, s.Tmt)
	// Restore the session table.
	st := sessstatesrv.RestoreTable(oldSt, mkps, rps, sesssrv, tmt, s.St)
	return dirover, ffs.(fs.Dir), stat.(*stats.Stats), st, tmt
}

func (s *Snapshot) RestoreFsTree(inum np.Tpath) fs.Inode {
	if obj, ok := s.restoreCache[inum]; ok {
		return obj
	}
	snap := s.Imap[inum]
	var i fs.Inode
	switch snap.Type {
	case Toverlay:
		// Make an overlay dir with a nil underlay so we don't recurse infinitely when trying
		// to set parent pointers.
		d := overlay.MkDirOverlay(nil)
		s.restoreCache[inum] = d
		i = overlay.Restore(d, s.RestoreFsTree, snap.Data)
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
	case Tfence:
		i = fencefs.RestoreFence(s.RestoreFsTree, snap.Data)
	case Tstats:
		i = stats.Restore(s.RestoreFsTree, snap.Data)
	case Tsnapshotdev:
		// Restore snapshot device
		i = RestoreSnapshotDev(s.RestoreFsTree, snap.Data)
	default:
		db.DFatalf("error unknown type in Snapshot.restore: %v", snap.Type)
		i = nil
	}
	// Store the object in the restore cache.
	s.restoreCache[inum] = i
	return i
}
