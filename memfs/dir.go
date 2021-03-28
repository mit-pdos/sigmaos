package memfs

import (
	"errors"
	"fmt"
	"sync"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npcodec"
	npo "ulambda/npobjsrv"
)

// Base("/") is "/", so check for "/" too. Base(".") is "." and Dir(".") is
// "." too
func IsCurrentDir(name string) bool {
	return name == "." || name == "/" || name == ""
}

type Dir struct {
	mu      sync.Mutex
	entries map[string]*Inode
}

func (dir *Dir) String() string {
	str := fmt.Sprintf("Dir{entries %v}", dir.entries)
	return str
}

func makeDir() *Dir {
	d := &Dir{}
	d.entries = make(map[string]*Inode)
	return d
}

func (dir *Dir) init(inodot *Inode) {
	dir.entries["."] = inodot
}

func (dir *Dir) removeLocked(name string) error {
	_, ok := dir.entries[name]
	if ok {
		delete(dir.entries, name)
		return nil
	}
	return fmt.Errorf("file not found %v", name)
}

func (dir *Dir) createLocked(ino *Inode, name string) error {
	_, ok := dir.entries[name]
	if ok {
		return errors.New("Name exists")
	}
	dir.entries[name] = ino
	return nil

}

func (dir *Dir) lookupLocked(name string) (*Inode, error) {
	inode, ok := dir.entries[name]
	if ok {
		return inode, nil
	} else {
		return nil, fmt.Errorf("file not found %v", name)
	}
}

func (dir *Dir) Len() np.Tlength {
	dir.mu.Lock()
	defer dir.mu.Unlock()
	return npcodec.DirSize(dir.lsL())
}

func (dir *Dir) namei(ctx npo.CtxI, path []string, inodes []npo.NpObj) ([]npo.NpObj, []string, error) {
	var inode *Inode
	var err error

	dir.mu.Lock()
	inode, err = dir.lookupLocked(path[0])
	if err != nil {
		db.DLPrintf("MEMFS", "dir %v: file not found %v", dir, path[0])
		dir.mu.Unlock()
		return nil, nil, err
	}
	inodes = append(inodes, inode)
	if inode.IsDir() {
		if len(path) == 1 { // done?
			db.DLPrintf("MEMFS", "namei %v %v -> %v", path, dir, inodes)
			dir.mu.Unlock()
			return inodes, nil, nil
		}
		d := inode.Data.(*Dir)
		dir.mu.Unlock() // for "."
		return d.namei(ctx, path[1:], inodes)
	} else {
		db.DLPrintf("MEMFS", "namei %v %v -> %v %v", path, dir, inodes, path[1:])
		dir.mu.Unlock()
		return inodes, path[1:], nil
	}
}

func (dir *Dir) lsL() []*np.Stat {
	entries := []*np.Stat{}
	for k, v := range dir.entries {
		if k == "." {
			continue
		}
		st, _ := v.statLocked()
		st.Name = k
		entries = append(entries, st)
	}
	return entries
}

func (dir *Dir) read(offset np.Toffset, cnt np.Tsize) ([]*np.Stat, error) {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	return dir.lsL(), nil
}

func (dir *Dir) create(inode *Inode, name string) error {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	return dir.createLocked(inode, name)
}

func (dir *Dir) remove(ctx npo.CtxI, name string) error {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	inode, err := dir.lookupLocked(name)
	if err != nil {
		db.DLPrintf("MEMFS", "remove %v file not found %v", dir, name)
		return err
	}
	if inode.IsDir() {
		d := inode.Data.(*Dir)
		if len(d.entries) <= 1 {
			fmt.Errorf("remove %v: not empty\n", name)
		}
	}
	return dir.removeLocked(name)
}
