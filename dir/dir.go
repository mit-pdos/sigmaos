package dir

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
	"ulambda/npcodec"
)

type makeInodeF func(fs.CtxI, np.Tperm, np.Tmode, fs.Dir) (fs.FsObj, *np.Err)
type makeRootInodeF func(fs.MakeDirF, fs.CtxI, np.Tperm) (fs.FsObj, *np.Err)
type genPathF func() np.Tpath

var makeInode makeInodeF
var genPath genPathF

// Base("/") is "/", so check for "/" too. Base(".") is "." and Dir(".") is
// "." too
func IsCurrentDir(name string) bool {
	return name == "." || name == "/" || name == ""
}

type DirImpl struct {
	fs.FsObj
	mu      sync.Mutex
	entries map[string]fs.FsObj
}

func MakeDir(i fs.FsObj) *DirImpl {
	d := &DirImpl{}
	d.FsObj = i
	d.entries = make(map[string]fs.FsObj)
	d.entries["."] = d
	return d
}

func MakeDirF(i fs.FsObj) fs.FsObj {
	d := MakeDir(i)
	return d
}

func (dir *DirImpl) String() string {
	str := fmt.Sprintf("dir %p i %p %T Dir{entries: ", dir, dir.FsObj, dir.FsObj)
	for n, e := range dir.entries {
		str += fmt.Sprintf("[%v %p]", n, e)
	}
	str += "}"
	return str
}

func MkRootDir(f makeInodeF, r makeRootInodeF, p genPathF) fs.Dir {
	makeInode = f
	genPath = p
	i, _ := r(MakeDirF, nil, np.DMDIR)
	return i.(fs.Dir)
}

func MkNod(ctx fs.CtxI, dir fs.Dir, name string, i fs.FsObj) *np.Err {
	err := dir.(*DirImpl).CreateDev(ctx, name, np.DMDEVICE, 0, i)
	if err != nil {
		return err
	}
	return nil
}

func (dir *DirImpl) unlinkL(name string) *np.Err {
	_, ok := dir.entries[name]
	if ok {
		delete(dir.entries, name)
		return nil
	}
	return np.MkErr(np.TErrNotfound, name)
}

func (dir *DirImpl) createL(ino fs.FsObj, name string) *np.Err {
	_, ok := dir.entries[name]
	if ok {
		return np.MkErr(np.TErrExists, name)
	}
	dir.entries[name] = ino
	return nil
}

func (dir *DirImpl) lookupL(name string) (fs.FsObj, *np.Err) {
	inode, ok := dir.entries[name]
	if ok {
		return inode, nil
	} else {
		return nil, np.MkErr(np.TErrNotfound, name)
	}
}

func (dir *DirImpl) Stat(ctx fs.CtxI) (*np.Stat, *np.Err) {
	dir.mu.Lock()
	defer dir.mu.Unlock()
	st, err := dir.FsObj.Stat(ctx)
	if err != nil {
		return nil, err
	}
	st.Length = npcodec.DirSize(dir.lsL(0))
	return st, nil
}

func (dir *DirImpl) Size() np.Tlength {
	dir.mu.Lock()
	defer dir.mu.Unlock()
	return npcodec.DirSize(dir.lsL(0))
}

func (dir *DirImpl) namei(ctx fs.CtxI, path []string, inodes []fs.FsObj) ([]fs.FsObj, []string, *np.Err) {
	var inode fs.FsObj
	var err *np.Err

	dir.mu.Lock()
	inode, err = dir.lookupL(path[0])
	if err != nil {
		db.DLPrintf("MEMFS", "dir %v: file not found %v", dir, path[0])
		dir.mu.Unlock()
		return inodes, path, err
	}
	inodes = append(inodes, inode)
	if len(path) == 1 { // done?
		db.DLPrintf("MEMFS", "namei %v dir %v -> %v", path, dir, inodes)
		dir.mu.Unlock()
		return inodes, nil, nil
	}
	switch i := inode.(type) {
	case *DirImpl:
		dir.mu.Unlock() // for "."
		return i.namei(ctx, path[1:], inodes)
	default:
		db.DLPrintf("MEMFS", "namei %T %v %v -> %v %v", i, path, dir, inodes, path[1:])
		dir.mu.Unlock()
		return inodes, path, np.MkErr(np.TErrNotDir, path[0])
	}
}

func (dir *DirImpl) lsL(cursor int) []*np.Stat {
	entries := []*np.Stat{}
	for k, v := range dir.entries {
		if k == "." {
			continue
		}
		st, _ := v.Stat(nil)
		st.Name = k
		entries = append(entries, st)
	}
	// sort dir by st.Name
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	if cursor > len(entries) {
		return nil
	} else {
		return entries[cursor:]
	}
}

func nonemptydir(inode fs.FsObj) bool {
	switch i := inode.(type) {
	case *DirImpl:
		if len(i.entries) > 1 {
			return true
		}
		return false
	default:
		return false
	}
}

func (dir *DirImpl) remove(name string) *np.Err {
	inode, err := dir.lookupL(name)
	if err != nil {
		db.DLPrintf("MEMFS", "remove %v file not found %v", dir, name)
		return err
	}
	if nonemptydir(inode) {
		return np.MkErr(np.TErrNotEmpty, name)
	}
	dir.VersionInc()
	dir.SetMtime(time.Now().Unix())
	return dir.unlinkL(name)
}

func (dir *DirImpl) Lookup(ctx fs.CtxI, path []string) ([]fs.FsObj, []string, *np.Err) {
	dir.mu.Lock()
	db.DLPrintf("MEMFS", "%v: Lookup %v %v\n", ctx, dir, path)
	dir.mu.Unlock()
	inodes := []fs.FsObj{}
	if len(path) == 0 {
		return nil, nil, nil
	}
	return dir.namei(ctx, path, inodes)
}

// XXX don't return more than n bytes of dir entries, since any more
// won't be sent to client anyway.
func (dir *DirImpl) ReadDir(ctx fs.CtxI, cursor int, n np.Tsize, v np.TQversion) ([]*np.Stat, *np.Err) {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	db.DLPrintf("MEMFS", "%v: ReadDir %v\n", ctx, dir)
	if !np.VEq(v, dir.Version()) {
		return nil, np.MkErr(np.TErrVersion, dir.Inum())
	}
	return dir.lsL(cursor), nil
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

	if IsCurrentDir(name) {
		return nil, np.MkErr(np.TErrInval, name)
	}
	newi, err := makeInode(ctx, perm, m, dir)
	if err != nil {
		return nil, err
	}
	db.DLPrintf("MEMFS", "Create %v in %v -> %v\n", name, dir, newi)
	dir.VersionInc()
	dir.SetMtime(time.Now().Unix())
	return newi, dir.createL(newi, name)
}

func (dir *DirImpl) CreateDev(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode, i fs.FsObj) *np.Err {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	if IsCurrentDir(name) {
		return np.MkErr(np.TErrIsdir, name)
	}
	db.DLPrintf("MEMFS", "CreateDev %v in %v -> %v\n", name, dir, i)
	dir.VersionInc()
	dir.SetMtime(time.Now().Unix())
	return dir.createL(i, name)
}

func lockOrdered(olddir *DirImpl, newdir *DirImpl) {
	id1 := olddir.Inum()
	id2 := newdir.Inum()
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
	id1 := olddir.Inum()
	id2 := newdir.Inum()
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

	db.DLPrintf("MEMFS", "%v: Rename %v -> %v\n", dir, from, to)
	ino, err := dir.lookupL(from)
	if err != nil {
		return err
	}

	// check if to is non-existing, or, if a dir, non-empty
	inoto, terr := dir.lookupL(to)
	if terr == nil && nonemptydir(inoto) {
		return np.MkErr(np.TErrNotEmpty, to)
	}

	err = dir.unlinkL(from)
	if err != nil {
		log.Fatalf("FATAL Rename: remove failed %v %v\n", from, err)
	}

	dir.VersionInc()
	if terr == nil { // inoto is valid
		// XXX 9p: it is an error to change the name to that
		// of an existing file.
		err = dir.remove(to)
		if err != nil {
			log.Fatalf("FATAL Rename remove failed %v %v\n", to, err)
		}
	}
	err = dir.createL(ino, to)
	if err != nil {
		log.Fatalf("FATAL Rename create %v failed %v\n", to, err)
		return err
	}
	ino.VersionInc()
	return nil

}

func (dir *DirImpl) Renameat(ctx fs.CtxI, old string, nd fs.Dir, new string) *np.Err {
	newdir := nd.(*DirImpl)
	lockOrdered(dir, newdir)
	defer unlockOrdered(dir, newdir)

	db.DLPrintf("MEMFS", "Renameat %v %v to %v %v\n", dir, old, newdir, new)
	ino, err := dir.lookupL(old)
	if err != nil {
		return np.MkErr(np.TErrNotfound, old)
	}
	err = dir.unlinkL(old)
	if err != nil {
		log.Fatalf("FATAL Rename %v remove  %v\n", old, err)
	}
	_, err = newdir.lookupL(new)
	if err == nil {
		err = newdir.remove(new)
	}
	err = newdir.createL(ino, new)
	if err != nil {
		log.Fatalf("FATAL Rename %v createL: %v\n", new, err)
		return err
	}
	// ino.VersionInc()
	ino.SetParent(newdir)
	return nil
}

func (dir *DirImpl) Remove(ctx fs.CtxI, n string) *np.Err {
	db.DLPrintf("MEMFS", "Remove: %v %v\n", dir, n)

	dir.mu.Lock()
	defer dir.mu.Unlock()

	inode, err := dir.lookupL(n)
	if err != nil {
		return err
	}

	inode.VersionInc()
	dir.VersionInc()

	err = dir.remove(n)
	return err
}

func (dir *DirImpl) Snapshot(fn fs.SnapshotF) []byte {
	return makeDirSnapshot(fn, dir)
}

func Restore(d *DirImpl, fn fs.RestoreF, b []byte) fs.FsObj {
	return restore(d, fn, b)
}
