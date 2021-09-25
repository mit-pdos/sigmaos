package fsimpl

import (
	"errors"
	"fmt"
	"log"

	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
	"ulambda/npcodec"
)

type makeInodeF func(string, np.Tperm, np.Tmode, fs.Dir) (fs.FsObj, error)

var makeInode makeInodeF

// Base("/") is "/", so check for "/" too. Base(".") is "." and Dir(".") is
// "." too
func IsCurrentDir(name string) bool {
	return name == "." || name == "/" || name == ""
}

type DirImpl struct {
	fs.FsObj
	entries map[string]fs.FsObj
}

func MakeDir(i fs.FsObj) *DirImpl {
	d := &DirImpl{}
	d.FsObj = i
	d.entries = make(map[string]fs.FsObj)
	d.entries["."] = d
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

func MkRootDir(f makeInodeF) fs.Dir {
	makeInode = f
	i := MakeInode("", np.DMDIR, nil)
	return MakeDir(i)
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
	dir.Lock()
	defer dir.Unlock()
	st, err := dir.FsObj.Stat(ctx)
	if err != nil {
		return nil, err
	}
	st.Length = npcodec.DirSize(dir.lsL())
	return st, nil
}

func (dir *DirImpl) Size() np.Tlength {
	dir.Lock()
	defer dir.Unlock()
	return npcodec.DirSize(dir.lsL())
}

func (dir *DirImpl) namei(ctx fs.CtxI, path []string, inodes []fs.FsObj) ([]fs.FsObj, []string, error) {
	var inode fs.FsObj
	var err error

	dir.Lock()
	inode, err = dir.lookupL(path[0])
	if err != nil {
		db.DLPrintf("MEMFS", "dir %v: file not found %v", dir, path[0])
		dir.Unlock()
		return nil, nil, err
	}
	inodes = append(inodes, inode)
	switch i := inode.(type) {
	case *DirImpl:
		if len(path) == 1 { // done?
			db.DLPrintf("MEMFS", "namei %v %v -> %v", path, dir, inodes)
			dir.Unlock()
			return inodes, nil, nil
		}
		dir.Unlock() // for "."
		return i.namei(ctx, path[1:], inodes)
	default:
		db.DLPrintf("MEMFS", "namei %v %v -> %v %v", path, dir, inodes, path[1:])
		dir.Unlock()
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
	dir.Lock()
	defer dir.Unlock()

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
	dir.SetMtime()
	return dir.removeL(name)
}

func (dir *DirImpl) Lookup(ctx fs.CtxI, path []string) ([]fs.FsObj, []string, error) {
	dir.Lock()
	db.DLPrintf("MEMFS", "%v: Lookup %v %v\n", ctx, dir, path)
	dir.Unlock()
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
	dir.Lock()
	defer dir.Unlock()

	db.DLPrintf("MEMFS", "%v: ReadDir %v\n", ctx, dir)
	if v != np.NoV && dir.Version() != v {
		return nil, fmt.Errorf("Version mismatch")
	}
	return dir.lsL(), nil
}

// XXX ax WriteDir from fs.Dir
func (dir *DirImpl) WriteDir(ctx fs.CtxI, offset np.Toffset, b []byte, v np.TQversion) (np.Tsize, error) {
	dir.Lock()
	defer dir.Unlock()
	return 0, errors.New("Cannot write directory")
}

func (dir *DirImpl) Create(ctx fs.CtxI, name string, perm np.Tperm, m np.Tmode) (fs.FsObj, error) {
	dir.Lock()
	defer dir.Unlock()

	if IsCurrentDir(name) {
		return nil, errors.New("Cannot create name")
	}
	newi, err := makeInode(ctx.Uname(), perm, m, dir)
	if err != nil {
		return nil, err
	}

	db.DLPrintf("MEMFS", "Create %v in %v -> %v\n", name, dir, newi)
	dir.VersionInc()
	dir.SetMtime()
	return newi, dir.createL(newi, name)
}

func lockOrdered(olddir *DirImpl, newdir *DirImpl) {
	id1 := olddir.Inum()
	id2 := newdir.Inum()
	if id1 == id2 {
		olddir.Lock()
	} else if id1 < id2 {
		olddir.Lock()
		newdir.Lock()
	} else {
		newdir.Lock()
		olddir.Lock()
	}
}

func unlockOrdered(olddir *DirImpl, newdir *DirImpl) {
	id1 := olddir.Inum()
	id2 := newdir.Inum()
	if id1 == id2 {
		olddir.Unlock()
	} else if id1 < id2 {
		olddir.Unlock()
		newdir.Unlock()
	} else {
		newdir.Unlock()
		olddir.Unlock()
	}
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
