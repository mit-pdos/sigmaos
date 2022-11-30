package overlay

import (
	"sync"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/inode"
	np "sigmaos/sigmap"
    "sigmaos/fcall"
)

//
// Overlay a directory with another directory transparently.  Servers
// can mount other file systems in the overlay directory (e.g.,
// statsd, fsfence, etc.).  This allows a server to export information
// to clients through sigmaP.
//

type DirOverlay struct {
	fs.Inode
	underlay fs.Dir
	mu       sync.Mutex
	entries  map[string]fs.Inode // XXX use sortedmap?
}

func MkDirOverlay(dir fs.Dir) *DirOverlay {
	d := &DirOverlay{}
	d.Inode = inode.MakeInode(nil, np.DMDIR, nil)
	d.underlay = dir
	d.entries = make(map[string]fs.Inode)
	return d
}

func (dir *DirOverlay) Mount(name string, i fs.Inode) {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	db.DPrintf("OVERLAYDIR", "Mount i %v as %v\n", i, name)

	dir.entries[name] = i
}

func (dir *DirOverlay) lookupMount(name string) fs.FsObj {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	db.DPrintf("OVERLAYDIR", "lookupMount %v %v\n", name, dir.entries)

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

// lookup in overlay
func (dir *DirOverlay) Lookup(ctx fs.CtxI, name string) (fs.FsObj, *fcall.Err) {
	if i := dir.lookupMount(name); i != nil {
		return i, nil
	}
	return nil, fcall.MkErr(fcall.TErrNotfound, name)
	// else {
	// 	db.DPrintf("OVERLAYDIR", "Lookup underlay %v\n", name)
	// 	o, err := dir.underlay.Lookup(ctx, name)
	// 	if o == dir.underlay {
	// 		o = dir
	// 	}
	// 	return o, err
	// }
}

func (dir *DirOverlay) LookupPath(ctx fs.CtxI, path np.Path) ([]fs.FsObj, fs.FsObj, np.Path, *fcall.Err) {
	if i := dir.lookupMount(path[0]); i != nil {
		return []fs.FsObj{i}, i, path[1:], nil
	} else {
		os, o, rest, err := dir.underlay.LookupPath(ctx, path)
		if o == dir.underlay {
			o = dir
		}
		return os, o, rest, err
	}
}

func (dir *DirOverlay) Create(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode) (fs.FsObj, *fcall.Err) {
	return dir.underlay.Create(ctx, name, perm, m)
}

// XXX account for extra entries in cursor, and sort
// XXX ignoressy size
func (dir *DirOverlay) ReadDir(ctx fs.CtxI, cursor int, n np.Tsize, v np.TQversion) ([]*np.Stat, *fcall.Err) {
	sts, err := dir.underlay.ReadDir(ctx, cursor, n, v)
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

func (dir *DirOverlay) WriteDir(ctx fs.CtxI, offset np.Toffset, b []byte, v np.TQversion) (np.Tsize, *fcall.Err) {
	return dir.underlay.WriteDir(ctx, offset, b, v)
}

func (dir *DirOverlay) Rename(ctx fs.CtxI, from, to string) *fcall.Err {
	return dir.underlay.Rename(ctx, from, to)
}

func (dir *DirOverlay) Renameat(ctx fs.CtxI, old string, nd fs.Dir, new string) *fcall.Err {
	return dir.underlay.Renameat(ctx, old, nd, new)
}

func (dir *DirOverlay) Remove(ctx fs.CtxI, n string) *fcall.Err {
	return dir.underlay.Remove(ctx, n)
}

// XXX only relevant if dir.underlay is *DirImpl. And, it already has been
// snapshotted.
func (dir *DirOverlay) Snapshot(fn fs.SnapshotF) []byte {
	return makeDirOverlaySnapshot(fn, dir)
}

func Restore(d *DirOverlay, fn fs.RestoreF, b []byte) fs.Inode {
	return restoreDirOverlay(d, fn, b)
}
