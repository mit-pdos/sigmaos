package dir

import (
	"fmt"
	"sync"
	"time"

	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
	"ulambda/npcodec"
	"ulambda/sorteddir"
)

type DirImpl struct {
	fs.Inode
	mi    fs.MakeInodeF
	mu    sync.Mutex
	dents *sorteddir.SortedDir
}

func MakeDir(i fs.Inode, mi fs.MakeInodeF) *DirImpl {
	d := &DirImpl{}
	d.Inode = i
	d.mi = mi
	d.dents = sorteddir.MkSortedDir()
	d.dents.Insert(".", d)
	return d
}

func MakeDirF(i fs.Inode, mi fs.MakeInodeF) fs.Inode {
	d := MakeDir(i, mi)
	return d
}

func (dir *DirImpl) String() string {
	str := fmt.Sprintf("dir %p i %p %T Dir{entries: ", dir, dir.Inode, dir.Inode)

	dir.dents.Iter(func(n string, e interface{}) bool {
		str += fmt.Sprintf("[%v]", n)
		return true
	})
	str += "}"
	return str
}

func MkRootDir(ctx fs.CtxI, mi fs.MakeInodeF) fs.Dir {
	i, _ := mi(ctx, np.DMDIR, 0, nil, MakeDirF)
	return i.(fs.Dir)
}

func MkNod(ctx fs.CtxI, dir fs.Dir, name string, i fs.Inode) *np.Err {
	err := dir.(*DirImpl).CreateDev(ctx, name, np.DMDEVICE, 0, i)
	if err != nil {
		return err
	}
	return nil
}

func (dir *DirImpl) unlinkL(name string) *np.Err {
	_, ok := dir.dents.Lookup(name)
	if ok {
		dir.dents.Delete(name)
		return nil
	}
	return np.MkErr(np.TErrNotfound, name)
}

func (dir *DirImpl) createL(ino fs.Inode, name string) *np.Err {
	ok := dir.dents.Insert(name, ino)
	if !ok {
		return np.MkErr(np.TErrExists, name)
	}
	return nil
}

func (dir *DirImpl) lookup(name string) (fs.Inode, *np.Err) {
	v, ok := dir.dents.Lookup(name)
	if ok {
		return v.(fs.Inode), nil
	} else {
		return nil, np.MkErr(np.TErrNotfound, name)
	}
}

func (dir *DirImpl) Lookup(ctx fs.CtxI, name string) (fs.FsObj, *np.Err) {
	v, ok := dir.dents.Lookup(name)
	if ok {
		return v.(fs.Inode), nil
	} else {
		return nil, np.MkErr(np.TErrNotfound, name)
	}
}

func (dir *DirImpl) Stat(ctx fs.CtxI) (*np.Stat, *np.Err) {
	dir.mu.Lock()
	defer dir.mu.Unlock()
	st, err := dir.Inode.Stat(ctx)
	if err != nil {
		return nil, err
	}
	sts, err := dir.lsL(0)
	if err != nil {
		return nil, err
	}
	st.Length = npcodec.MarshalSizeDir(sts)
	return st, nil
}

func (dir *DirImpl) Size() (np.Tlength, *np.Err) {
	dir.mu.Lock()
	defer dir.mu.Unlock()
	sts, err := dir.lsL(0)
	if err != nil {
		return 0, err
	}
	return npcodec.MarshalSizeDir(sts), nil
}

func (dir *DirImpl) lsL(cursor int) ([]*np.Stat, *np.Err) {
	entries := []*np.Stat{}
	var r *np.Err
	dir.dents.Iter(func(n string, e interface{}) bool {
		if n == "." {
			return true
		}
		i := e.(fs.Inode)
		st, err := i.Stat(nil)
		if err != nil {
			r = err
			return false
		}
		st.Name = n
		entries = append(entries, st)
		return true
	})
	if r != nil {
		return nil, r
	}
	if cursor > len(entries) {
		return nil, nil
	} else {
		return entries[cursor:], nil
	}
}

func nonemptydir(inode fs.FsObj) bool {
	switch i := inode.(type) {
	case *DirImpl:
		if i.dents.Len() > 1 {
			return true
		}
		return false
	default:
		return false
	}
}

func (dir *DirImpl) remove(name string) *np.Err {
	inode, err := dir.lookup(name)
	if err != nil {
		db.DPrintf("MEMFS", "remove %v file not found %v", dir, name)
		return err
	}
	if nonemptydir(inode) {
		return np.MkErr(np.TErrNotEmpty, name)
	}
	dir.SetMtime(time.Now().Unix())
	return dir.unlinkL(name)
}

// XXX don't return more than n bytes of dir entries, since any more
// won't be sent to client anyway.
func (dir *DirImpl) ReadDir(ctx fs.CtxI, cursor int, n np.Tsize, v np.TQversion) ([]*np.Stat, *np.Err) {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	db.DPrintf("MEMFS", "%v: ReadDir %v\n", ctx, dir)
	return dir.lsL(cursor)
}

// XXX ax WriteDir from fs.Dir
func (dir *DirImpl) WriteDir(ctx fs.CtxI, offset np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	dir.mu.Lock()
	defer dir.mu.Unlock()
	return 0, np.MkErr(np.TErrIsdir, dir)
}

func (dir *DirImpl) Create(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode) (fs.FsObj, *np.Err) {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	if v, ok := dir.dents.Lookup(name); ok {
		i := v.(fs.Inode)
		return i, np.MkErr(np.TErrExists, name)
	}
	newi, err := dir.mi(ctx, perm, m, dir, MakeDirF)
	if err != nil {
		return nil, err
	}
	db.DPrintf("MEMFS", "Create %v in %v -> %v\n", name, dir, newi)
	dir.SetMtime(time.Now().Unix())
	return newi, dir.createL(newi, name)
}

func (dir *DirImpl) CreateDev(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode, i fs.Inode) *np.Err {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	db.DPrintf("MEMFS", "CreateDev %v in %v -> %v\n", name, dir, i)
	dir.SetMtime(time.Now().Unix())
	return dir.createL(i, name)
}

func lockOrdered(olddir *DirImpl, newdir *DirImpl) {
	id1 := olddir.Path()
	id2 := newdir.Path()
	if id1 == id2 {
		olddir.mu.Lock()
	} else if id1 < id2 {
		olddir.mu.Lock()
		newdir.mu.Lock()
	} else {
		newdir.mu.Lock()
		olddir.mu.Lock()
	}
}

func unlockOrdered(olddir *DirImpl, newdir *DirImpl) {
	id1 := olddir.Path()
	id2 := newdir.Path()
	if id1 == id2 {
		olddir.mu.Unlock()
	} else if id1 < id2 {
		olddir.mu.Unlock()
		newdir.mu.Unlock()
	} else {
		newdir.mu.Unlock()
		olddir.mu.Unlock()
	}
}

// Rename inode within directory
func (dir *DirImpl) Rename(ctx fs.CtxI, from, to string) *np.Err {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	db.DPrintf("MEMFS", "%v: Rename %v -> %v\n", dir, from, to)
	ino, err := dir.lookup(from)
	if err != nil {
		return err
	}

	// check if to is non-existing, or, if a dir, non-empty
	inoto, terr := dir.lookup(to)
	if terr == nil && nonemptydir(inoto) {
		return np.MkErr(np.TErrNotEmpty, to)
	}

	err = dir.unlinkL(from)
	if err != nil {
		db.DFatalf("Rename: remove failed %v %v\n", from, err)
	}

	if terr == nil { // inoto is valid
		// XXX 9p: it is an error to change the name to that
		// of an existing file.
		err = dir.remove(to)
		if err != nil {
			db.DFatalf("Rename remove failed %v %v\n", to, err)
		}
	}
	err = dir.createL(ino, to)
	if err != nil {
		db.DFatalf("Rename create %v failed %v\n", to, err)
		return err
	}
	return nil

}

func (dir *DirImpl) Renameat(ctx fs.CtxI, old string, nd fs.Dir, new string) *np.Err {
	newdir := nd.(*DirImpl)
	lockOrdered(dir, newdir)
	defer unlockOrdered(dir, newdir)

	db.DPrintf("MEMFS", "Renameat %v %v to %v %v\n", dir, old, newdir, new)
	ino, err := dir.lookup(old)
	if err != nil {
		return np.MkErr(np.TErrNotfound, old)
	}
	err = dir.unlinkL(old)
	if err != nil {
		db.DFatalf("Rename %v remove  %v\n", old, err)
	}
	_, err = newdir.lookup(new)
	if err == nil {
		err = newdir.remove(new)
	}
	err = newdir.createL(ino, new)
	if err != nil {
		db.DFatalf("Rename %v createL: %v\n", new, err)
		return err
	}
	ino.SetParent(newdir)
	return nil
}

func (dir *DirImpl) Remove(ctx fs.CtxI, n string) *np.Err {
	db.DPrintf("MEMFS", "Remove: %v %v\n", dir, n)

	dir.mu.Lock()
	defer dir.mu.Unlock()

	inode, err := dir.lookup(n)
	if err != nil {
		return err
	}
	if err := dir.remove(n); err != nil {
		return err
	}
	inode.Unlink()
	return nil
}

func (dir *DirImpl) Snapshot(fn fs.SnapshotF) []byte {
	return makeDirSnapshot(fn, dir)
}

func Restore(d *DirImpl, fn fs.RestoreF, b []byte) fs.Inode {
	return restore(d, fn, b)
}
