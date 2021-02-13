package memfs

import (
	"errors"
	"fmt"
	"log"
	"sync"

	db "ulambda/debug"
	np "ulambda/ninep"
	"ulambda/npcodec"
)

// Base("/") is "/", so check for "/" too. Base(".") is "." and Dir(".") is
// "." too
func IsCurrentDir(name string) bool {
	return name == "." || name == "/" || name == ""
}

type Dir struct {
	mu      sync.Mutex
	inum    Tinum
	entries map[string]*Inode
}

func (dir *Dir) String() string {
	str := fmt.Sprintf("Dir{%v entries %v}", dir.inum, dir.entries)
	return str
}

func makeDir() *Dir {
	d := &Dir{}
	d.entries = make(map[string]*Inode)

	return d
}

func (dir *Dir) init(inodot *Inode) {
	dir.inum = inodot.Inum
	dir.entries["."] = inodot
}

func (dir *Dir) removeLocked(name string) error {
	_, ok := dir.entries[name]
	if ok {
		delete(dir.entries, name)
		return nil
	}
	return fmt.Errorf("Unknown name %v", name)
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
		return nil, fmt.Errorf("Unknown name %v", name)
	}
}

func (dir *Dir) Len() np.Tlength {
	dir.mu.Lock()
	defer dir.mu.Unlock()
	return npcodec.DirSize(dir.lsL())
}

func (dir *Dir) namei(uname string, path []string, inodes []*Inode) ([]*Inode, []string, error) {
	var inode *Inode
	var err error

	dir.mu.Lock()
	if dir.inum == 0 {
		log.Fatal("namei ", dir)
	}
	inode, err = dir.lookupLocked(path[0])
	if err != nil {
		db.DPrintf("%v: namei %v unknown %v", uname, dir, path)
		dir.mu.Unlock()
		return nil, nil, err
	}
	inodes = append(inodes, inode)
	if inode.IsDir() {
		if len(path) == 1 { // done?
			db.DPrintf("namei %v %v -> %v", path, dir, inodes)
			dir.mu.Unlock()
			return inodes, nil, nil
		}
		d := inode.Data.(*Dir)
		dir.mu.Unlock() // for "."
		return d.namei(uname, path[1:], inodes)
	} else {
		db.DPrintf("namei %v %v -> %v %v", path, dir, inodes, path[1:])
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
		st := v.Stat()
		st.Name = k
		entries = append(entries, st)
	}
	return entries
}

func (dir *Dir) read(offset np.Toffset, cnt np.Tsize) ([]byte, error) {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	entries := dir.lsL()
	return npcodec.Dir2Byte(offset, cnt, entries)
}

func (dir *Dir) create(inode *Inode, name string) error {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	return dir.createLocked(inode, name)
}

func (dir *Dir) remove(name string) error {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	inode, err := dir.lookupLocked(name)
	if err != nil {
		db.DPrintf("remove %v unknown %v", dir, name)
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
