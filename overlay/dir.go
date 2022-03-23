package overlay

import (
	"sync"

	// "ulambda/ctx"
	db "ulambda/debug"
	"ulambda/fs"
	// "ulambda/inode"
	np "ulambda/ninep"
	// "ulambda/npcodec"
)

type DirOverlay struct {
	fs.Dir
	mu      sync.Mutex
	entries map[string]fs.FsObj
}

func MkDirOverlay(dir fs.Dir) *DirOverlay {
	d := &DirOverlay{}
	d.Dir = dir
	d.entries = make(map[string]fs.FsObj)
	return d
}

func (dir *DirOverlay) Mount(name string, i fs.FsObj) {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	db.DLPrintf("OVERLAYDIR", "Mount i %v as %v\n", i, name)

	dir.entries[name] = i
}

func (dir *DirOverlay) lookupMount(name string) fs.FsObj {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	db.DLPrintf("OVERLAYDIR", "lookupMount %v %v\n", name, dir.entries)

	if i, ok := dir.entries[name]; ok {
		return i
	}
	return nil
}

func (dir *DirOverlay) ls() []*np.Stat {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	entries := make([]*np.Stat, 0, len(dir.entries))
	for k, i := range dir.entries {
		st, _ := i.Stat(nil)
		st.Name = k
		entries = append(entries, st)
	}
	return entries
}

// path is in overlay mounts
func (dir *DirOverlay) lookup(ctx fs.CtxI, path np.Path) ([]np.Tqid, fs.FsObj, np.Path, *np.Err) {
	i := dir.lookupMount(path[0])
	qids := []np.Tqid{i.Qid()}
	db.DLPrintf("OVERLAYDIR", "lookup %v in mount %v %v\n", path[1:], path[0], i)
	if len(path) == 1 {
		return qids, i, nil, nil
	} else {
		switch d := i.(type) {
		case fs.Dir:
			qs, lo, p, err := d.Lookup(ctx, path[1:])
			db.DLPrintf("OVERLAYDIR", "lookup %v in %v res %v %v %v err %v\n", path[1:], i, qs, lo, p, err)
			if lo == nil {
				lo = i
			}
			return append(qids, qs...), lo, p, err
		default:
			return qids, i, path[1:], np.MkErr(np.TErrNotDir, path[0])
		}
	}
}

func (dir *DirOverlay) Lookup(ctx fs.CtxI, path np.Path) ([]np.Tqid, fs.FsObj, np.Path, *np.Err) {
	if len(path) >= 1 && dir.lookupMount(path[0]) != nil {
		// lookup up in overlay
		return dir.lookup(ctx, path)
	} else {
		db.DLPrintf("OVERLAYDIR", "Lookup underlay %v\n", path)
		// lookup up in underlay
		qids, lo, p, err := dir.Dir.Lookup(ctx, path)
		if lo == dir.Dir {
			lo = dir
		}
		return qids, lo, p, err
	}
}

func (dir *DirOverlay) Create(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode) (fs.FsObj, *np.Err) {
	return dir.Dir.Create(ctx, name, perm, m)
}

// XXX account for extra entries in cursor, and sort
// XXX ignoressy size
func (dir *DirOverlay) ReadDir(ctx fs.CtxI, cursor int, n np.Tsize, v np.TQversion) ([]*np.Stat, *np.Err) {
	sts, err := dir.Dir.ReadDir(ctx, cursor, n, v)
	if err != nil {
		return nil, err
	}
	if cursor > 0 { // did we already sent the extra ones
		return sts, err
	}
	// prepend the extra ones
	sts = append(dir.ls(), sts...)
	return sts, nil
}

func (dir *DirOverlay) WriteDir(ctx fs.CtxI, offset np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	return dir.Dir.WriteDir(ctx, offset, b, v)
}

func (dir *DirOverlay) Rename(ctx fs.CtxI, from, to string) *np.Err {
	return dir.Dir.Rename(ctx, from, to)
}

func (dir *DirOverlay) Renameat(ctx fs.CtxI, old string, nd fs.Dir, new string) *np.Err {
	return dir.Dir.Renameat(ctx, old, nd, new)
}

func (dir *DirOverlay) Remove(ctx fs.CtxI, n string) *np.Err {
	return dir.Dir.Remove(ctx, n)
}

// XXX only relevant if dir.Dir is *DirImpl. And, it already has been
// snapshotted.
func (dir *DirOverlay) Snapshot(fn fs.SnapshotF) []byte {
	return nil
}

func Restore(d *DirOverlay, fn fs.RestoreF, b []byte) fs.Inode {
	return nil
}
