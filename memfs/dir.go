package memfs

import (
	"errors"
	"fmt"
	"log"
	"time"

	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
	"ulambda/npcodec"
)

// Base("/") is "/", so check for "/" too. Base(".") is "." and Dir(".") is
// "." too
func IsCurrentDir(name string) bool {
	return name == "." || name == "/" || name == ""
}

type Dir struct {
	*Inode
	entries map[string]InodeI
}

func makeDir(i *Inode) *Dir {
	d := &Dir{}
	d.Inode = i
	d.entries = make(map[string]InodeI)
	d.entries["."] = d
	return d
}

func (dir *Dir) String() string {
	str := "Dir{entries: "
	for n, e := range dir.entries {
		if n != "." {
			str += fmt.Sprintf("[%v %p]", n, e)
		}
	}
	str += "}"
	return str
}

func MkRootInode() *Dir {
	i := makeInode("", np.DMDIR, nil)
	return makeDir(i)
}

func (dir *Dir) removeL(name string) error {
	_, ok := dir.entries[name]
	if ok {
		delete(dir.entries, name)
		return nil
	}
	return fmt.Errorf("file not found %v", name)
}

func (dir *Dir) createL(ino InodeI, name string) error {
	_, ok := dir.entries[name]
	if ok {
		return errors.New("Name exists")
	}
	dir.entries[name] = ino
	return nil

}

func (dir *Dir) lookupL(name string) (InodeI, error) {
	inode, ok := dir.entries[name]
	if ok {
		return inode, nil
	} else {
		return nil, fmt.Errorf("file not found %v", name)
	}
}

func (dir *Dir) Stat(ctx fs.CtxI) (*np.Stat, error) {
	dir.Lock()
	defer dir.Unlock()
	st := dir.Inode.stat()
	st.Length = npcodec.DirSize(dir.lsL())
	return st, nil
}

func (dir *Dir) Size() np.Tlength {
	dir.Lock()
	defer dir.Unlock()
	return npcodec.DirSize(dir.lsL())
}

func (dir *Dir) SetParent(p *Dir) {
	dir.parent = p
}

func (dir *Dir) namei(ctx fs.CtxI, path []string, inodes []fs.NpObj) ([]fs.NpObj, []string, error) {
	var inode InodeI
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
	case *Dir:
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

func (dir *Dir) lsL() []*np.Stat {
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

func (dir *Dir) remove(ctx fs.CtxI, name string) error {
	dir.Lock()
	defer dir.Unlock()

	inode, err := dir.lookupL(name)
	if err != nil {
		db.DLPrintf("MEMFS", "remove %v file not found %v", dir, name)
		return err
	}
	switch i := inode.(type) {
	case *Dir:
		if len(i.entries) > 1 {
			return fmt.Errorf("remove %v: not empty\n", name)
		}
	default:
	}
	dir.version += 1
	dir.Mtime = time.Now().Unix()
	return dir.removeL(name)
}

func (dir *Dir) CreateDev(ctx fs.CtxI, name string, d Dev, t np.Tperm, m np.Tmode) (fs.NpObj, error) {
	dir.Lock()
	defer dir.Unlock()

	if IsCurrentDir(name) {
		return nil, errors.New("Cannot create name")
	}
	newi, err := permToInode(ctx.Uname(), t, dir)
	if err != nil {
		return nil, err
	}
	if d != nil {
		dev := newi.(*Device)
		dev.d = d
	}
	db.DLPrintf("MEMFS", "Create %v in %v -> %v\n", name, dir, newi)
	dir.version += 1
	dir.Mtime = time.Now().Unix()
	return newi, dir.createL(newi, name)
}

func (dir *Dir) Create(ctx fs.CtxI, name string, t np.Tperm, m np.Tmode) (fs.NpObj, error) {
	return dir.CreateDev(ctx, name, nil, t, m)
}

func (dir *Dir) Lookup(ctx fs.CtxI, path []string) ([]fs.NpObj, []string, error) {
	dir.Lock()
	db.DLPrintf("MEMFS", "%v: Lookup %v %v\n", ctx, dir, path)
	dir.Unlock()
	inodes := []fs.NpObj{}
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

func (dir *Dir) ReadDir(ctx fs.CtxI, offset np.Toffset, n np.Tsize, v np.TQversion) ([]*np.Stat, error) {
	dir.Lock()
	defer dir.Unlock()

	db.DLPrintf("MEMFS", "%v: ReadDir %v\n", ctx, dir)
	if v != np.NoV && dir.version != v {
		return nil, fmt.Errorf("Version mismatch")
	}

	return dir.lsL(), nil
}
func (inode *Inode) WriteDir(ctx fs.CtxI, offset np.Toffset, b []byte, v np.TQversion) (np.Tsize, error) {
	return 0, errors.New("Cannot write directory")
}

func lockOrdered(olddir *Dir, newdir *Dir) {
	id1 := olddir.Id()
	id2 := newdir.Id()
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

func unlockOrdered(olddir *Dir, newdir *Dir) {
	id1 := olddir.Id()
	id2 := newdir.Id()
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

func (dir *Dir) Renameat(ctx fs.CtxI, old string, nd fs.NpObjDir, new string) error {
	newdir := nd.(*Dir)
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
