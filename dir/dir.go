package dir

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
	"ulambda/npcodec"
)

type makeInodeF func(string, np.Tperm, np.Tmode, fs.Dir) (fs.FsObj, error)
type makeRootInodeF func(fs.MakeDirF, string, np.Tperm) (fs.FsObj, error)

var makeInode makeInodeF

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
	str := "Dir{entries: "
	for n, e := range dir.entries {
		if n != "." {
			str += fmt.Sprintf("[%v %p]", n, e)
		}
	}
	str += "}"
	return str
}

func MkRootDir(f makeInodeF, r makeRootInodeF) fs.Dir {
	makeInode = f
	i, _ := r(MakeDirF, "", np.DMDIR)
	return MakeDir(i)
}

func MkNod(ctx fs.CtxI, dir fs.Dir, name string, i fs.FsObj) error {
	err := dir.(*DirImpl).CreateDev(ctx, name, np.DMDEVICE, 0, i)
	if err != nil {
		return err
	}
	return nil
}

func (dir *DirImpl) removeL(name string) error {
	_, ok := dir.entries[name]
	if ok {
		delete(dir.entries, name)
		return nil
	}
	return fmt.Errorf("file not found %v", name)
}

func (dir *DirImpl) createL(ino fs.FsObj, name string) error {
	_, ok := dir.entries[name]
	if ok {
		return errors.New("Name exists")
	}
	dir.entries[name] = ino
	return nil
}

func (dir *DirImpl) lookupL(name string) (fs.FsObj, error) {
	inode, ok := dir.entries[name]
	if ok {
		return inode, nil
	} else {
		return nil, fmt.Errorf("file not found %v", name)
	}
}

func (dir *DirImpl) Stat(ctx fs.CtxI) (*np.Stat, error) {
	dir.mu.Lock()
	defer dir.mu.Unlock()
	st, err := dir.FsObj.Stat(ctx)
	if err != nil {
		return nil, err
	}
	st.Length = npcodec.DirSize(dir.lsL())
	return st, nil
}

func (dir *DirImpl) Size() np.Tlength {
	dir.mu.Lock()
	defer dir.mu.Unlock()
	return npcodec.DirSize(dir.lsL())
}

func (dir *DirImpl) namei(ctx fs.CtxI, path []string, inodes []fs.FsObj) ([]fs.FsObj, []string, error) {
	var inode fs.FsObj
	var err error

	dir.mu.Lock()
	inode, err = dir.lookupL(path[0])
	if err != nil {
		db.DLPrintf("MEMFS", "dir %v: file not found %v", dir, path[0])
		dir.mu.Unlock()
		return nil, nil, err
	}
	inodes = append(inodes, inode)
	switch i := inode.(type) {
	case *DirImpl:
		if len(path) == 1 { // done?
			db.DLPrintf("MEMFS", "namei %v %v -> %v", path, dir, inodes)
			dir.mu.Unlock()
			return inodes, nil, nil
		}
		dir.mu.Unlock() // for "."
		return i.namei(ctx, path[1:], inodes)
	default:
		db.DLPrintf("MEMFS", "namei %v %v -> %v %v", path, dir, inodes, path[1:])
		dir.mu.Unlock()
		return inodes, path[1:], nil
	}
}

func (dir *DirImpl) lsL() []*np.Stat {
	entries := []*np.Stat{}
	for k, v := range dir.entries {
		if k == "." {
			continue
		}
		st, _ := v.Stat(nil)
		st.Name = k
		entries = append(entries, st)
	}
	return entries
}

func (dir *DirImpl) remove(ctx fs.CtxI, name string) error {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	inode, err := dir.lookupL(name)
	if err != nil {
		db.DLPrintf("MEMFS", "remove %v file not found %v", dir, name)
		return err
	}
	switch i := inode.(type) {
	case *DirImpl:
		if len(i.entries) > 1 {
			return fmt.Errorf("remove %v: not empty\n", name)
		}
	default:
	}
	dir.VersionInc()
	dir.SetMtime(time.Now().Unix())
	return dir.removeL(name)
}

func (dir *DirImpl) Lookup(ctx fs.CtxI, path []string) ([]fs.FsObj, []string, error) {
	dir.mu.Lock()
	db.DLPrintf("MEMFS", "%v: Lookup %v %v\n", ctx, dir, path)
	dir.mu.Unlock()
	inodes := []fs.FsObj{}
	if len(path) == 0 {
		return nil, nil, nil
	}
	db.DLPrintf("MEMFS", "lookup: %v\n", path)
	inodes, rest, err := dir.namei(ctx, path, inodes)
	if err == nil {
		return inodes, rest, err
	} else {
		return nil, rest, err // XXX was nil?
	}
}

func (dir *DirImpl) ReadDir(ctx fs.CtxI, offset np.Toffset, n np.Tsize, v np.TQversion) ([]*np.Stat, error) {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	db.DLPrintf("MEMFS", "%v: ReadDir %v\n", ctx, dir)
	if v != np.NoV && dir.Version() != v {
		return nil, fmt.Errorf("Version mismatch")
	}
	return dir.lsL(), nil
}

// XXX ax WriteDir from fs.Dir
func (dir *DirImpl) WriteDir(ctx fs.CtxI, offset np.Toffset, b []byte, v np.TQversion) (np.Tsize, error) {
	dir.mu.Lock()
	defer dir.mu.Unlock()
	return 0, errors.New("Cannot write directory")
}

func (dir *DirImpl) Create(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode) (fs.FsObj, error) {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	if IsCurrentDir(name) {
		return nil, errors.New("Cannot create name")
	}
	newi, err := makeInode(ctx.Uname(), perm, m, dir)
	if err != nil {
		return nil, err
	}

	db.DLPrintf("MEMFS", "Create %v in %v -> %v\n", name, dir, newi)
	dir.VersionInc()
	dir.SetMtime(time.Now().Unix())
	return newi, dir.createL(newi, name)
}

func (dir *DirImpl) CreateDev(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode, i fs.FsObj) error {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	if IsCurrentDir(name) {
		return errors.New("Cannot create name")
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
func (dir *DirImpl) Rename(ctx fs.CtxI, from, to string) error {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	db.DLPrintf("MEMFS", "%v: Rename %v -> %v\n", dir, from, to)
	ino, err := dir.lookupL(from)
	if err != nil {
		return err
	}
	err = dir.removeL(from)
	if err != nil {
		log.Fatalf("Rename: remove failed %v %v\n", from, err)
	}
	_, err = dir.lookupL(to)
	if err == nil { // i is valid
		// XXX 9p: it is an error to change the name to that
		// of an existing file.
		err = dir.removeL(to)
		if err != nil {
			log.Fatalf("Rename remove failed %v %v\n", to, err)
		}
	}
	err = dir.createL(ino, to)
	if err != nil {
		log.Fatalf("Rename create %v failed %v\n", to, err)
		return err
	}
	return nil

}

func (dir *DirImpl) Renameat(ctx fs.CtxI, old string, nd fs.Dir, new string) error {
	newdir := nd.(*DirImpl)
	lockOrdered(dir, newdir)
	defer unlockOrdered(dir, newdir)

	db.DLPrintf("MEMFS", "Renameat %v %v to %v %v\n", dir, old, newdir, new)
	ino, err := dir.lookupL(old)
	if err != nil {
		return fmt.Errorf("file not found %v", old)
	}
	err = dir.removeL(old)
	if err != nil {
		log.Fatalf("Rename %v remove  %v\n", old, err)
	}
	_, err = newdir.lookupL(new)
	if err == nil {
		err = newdir.removeL(new)
	}
	err = newdir.createL(ino, new)
	if err != nil {
		log.Fatalf("Rename %v createL: %v\n", new, err)
		return err
	}
	ino.SetParent(newdir)
	return nil
}

func (dir *DirImpl) Remove(ctx fs.CtxI, n string) error {
	db.DLPrintf("MEMFS", "Remove: %v %v\n", dir, n)

	dir.mu.Lock()
	defer dir.mu.Unlock()

	inode, err := dir.lookupL(n)
	if err != nil {
		return err
	}

	inode.VersionInc()
	dir.VersionInc()

	err = dir.removeL(n)
	if err != nil {
		log.Fatalf("Remove: error %v\n", n)
	}

	return nil
}
