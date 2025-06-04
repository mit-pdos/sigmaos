// The overlay package overlays a directory on top of another
// directory, the underlay, transparently.  Servers can mount other
// file systems in the overlay directory (e.g., statsd, fsfence,
// etc.).  This allows a server to export information to clients
// through sigmaP.
package overlaydir

import (
	"sync"

	"sigmaos/api/fs"
	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type DirOverlay struct {
	underlay fs.Dir
	mu       sync.Mutex
	entries  map[string]fs.FsObj // XXX use sortedmap?
}

func MkDirOverlay(dir fs.Dir) *DirOverlay {
	d := &DirOverlay{}
	d.underlay = dir
	d.entries = make(map[string]fs.FsObj)
	return d
}

func (dir *DirOverlay) String() string {
	return dir.underlay.String()
}

func (dir *DirOverlay) Path() sp.Tpath {
	// return dir.i.Path()
	return dir.underlay.Path()
}

func (dir *DirOverlay) Perm() sp.Tperm {
	return dir.underlay.Perm()
}

func (dir *DirOverlay) IsLeased() bool {
	return dir.underlay.IsLeased()
}

func (dir *DirOverlay) SetMtime(t int64) {
	dir.underlay.SetMtime(t)
}

func (dir *DirOverlay) Mtime() int64 {
	return dir.underlay.Mtime()
}

func (dir *DirOverlay) Unlink() {
	dir.underlay.Unlink()
}

func (dir *DirOverlay) NewStat() (*sp.Tstat, *serr.Err) {
	return dir.underlay.NewStat()
}

func (dir *DirOverlay) Stat(ctx fs.CtxI) (*sp.Tstat, *serr.Err) {
	st, err := dir.underlay.NewStat()
	if err != nil {
		return nil, err
	}
	st.SetLength(st.Tlength() + sp.Tlength(len(dir.entries)))
	return st, nil
}

func (dir *DirOverlay) Mount(name string, i fs.FsObj) {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	db.DPrintf(db.OVERLAYDIR, "Mount i %v as %v\n", i, name)

	if dir.Path() == i.Path() {
		db.DFatalf("Mount: path %v %v are equal", dir.Path(), i.Path())
	}
	dir.entries[name] = i
}

func (dir *DirOverlay) lookupMount(name string) fs.FsObj {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	db.DPrintf(db.OVERLAYDIR, "lookupMount %v %v\n", name, dir.entries)

	if i, ok := dir.entries[name]; ok {
		return i
	}
	return nil
}

func (dir *DirOverlay) removeMount(name string) bool {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	db.DPrintf(db.OVERLAYDIR, "removeMount %v %v\n", name, dir.entries)

	if _, ok := dir.entries[name]; ok {
		delete(dir.entries, name)
		return true
	}
	return false
}

func (dir *DirOverlay) ls(ctx fs.CtxI) ([]*sp.Tstat, error) {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	entries := make([]*sp.Tstat, 0, len(dir.entries))
	for k, i := range dir.entries {
		st, err := i.Stat(ctx)
		if err != nil {
			return nil, err
		}
		st.Name = k
		entries = append(entries, st)
	}
	return entries, nil
}

// lookup in overlay
func (dir *DirOverlay) Lookup(ctx fs.CtxI, name string) (fs.FsObj, *serr.Err) {
	if i := dir.lookupMount(name); i != nil {
		return i, nil
	}
	return nil, serr.NewErr(serr.TErrNotfound, name)
}

func (dir *DirOverlay) LookupPath(ctx fs.CtxI, path path.Tpathname) ([]fs.FsObj, fs.FsObj, path.Tpathname, *serr.Err) {
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

func (dir *DirOverlay) Create(ctx fs.CtxI, name string, perm sp.Tperm, m sp.Tmode, lid sp.TleaseId, f sp.Tfence, d fs.FsObj) (fs.FsObj, *serr.Err) {
	if i := dir.lookupMount(name); i != nil {
		return i, serr.NewErr(serr.TErrExists, name)
	}
	return dir.underlay.Create(ctx, name, perm, m, lid, f, nil)
}

func (dir *DirOverlay) Open(ctx fs.CtxI, m sp.Tmode) (fs.FsObj, *serr.Err) {
	return dir.underlay.Open(ctx, m)
}

func (dir *DirOverlay) Close(ctx fs.CtxI, m sp.Tmode) *serr.Err {
	return dir.underlay.Close(ctx, m)
}

// XXX account for extra entries in cursor, and sort
// XXX ignores size
func (dir *DirOverlay) ReadDir(ctx fs.CtxI, cursor int, n sp.Tsize) ([]*sp.Tstat, *serr.Err) {
	sts, err := dir.underlay.ReadDir(ctx, cursor, n)
	if err != nil {
		return nil, err
	}
	db.DPrintf(db.OVERLAYDIR, "ReadDir underlay %v cursor %d %v\n", dir, cursor, sp.Names(sts))
	if cursor > 0 { // did we already sent the extra ones
		return sts, err
	}
	// prepend the extra ones
	stso, r := dir.ls(ctx)
	if r != nil {
		return nil, err
	}
	sts = append(stso, sts...)
	return sts, nil
}

func (dir *DirOverlay) Rename(ctx fs.CtxI, from, to string, f sp.Tfence) *serr.Err {
	if i := dir.lookupMount(from); i != nil {
		return serr.NewErr(serr.TErrNotSupported, from)
	}
	return dir.underlay.Rename(ctx, from, to, f)
}

func (dir *DirOverlay) Renameat(ctx fs.CtxI, old string, nd fs.Dir, new string, f sp.Tfence) *serr.Err {
	if i := dir.lookupMount(old); i != nil {
		return serr.NewErr(serr.TErrNotSupported, old)
	}
	return dir.underlay.Renameat(ctx, old, nd, new, f)
}

func (dir *DirOverlay) Remove(ctx fs.CtxI, n string, f sp.Tfence, del fs.Tdel) *serr.Err {
	if dir.removeMount(n) {
		return nil
	}
	return dir.underlay.Remove(ctx, n, f, del)
}
