package memfs

import (
	"errors"
	"fmt"
	"log"
	"sync"

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
	inum    Tinum // for acquiring locks in rename()
	entries map[string]*Inode
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

func (dir *Dir) lenLocked() np.Tlength {
	sz := uint32(0)
	for n, i := range dir.entries {
		if n != "." {
			st := *i.Stat()
			st.Name = n
			sz += npcodec.SizeNp(st)
		}
	}
	return np.Tlength(sz)
}

func (dir *Dir) removeLocked(name string) error {
	_, ok := dir.entries[name]
	if ok {
		delete(dir.entries, name)
		return nil
	}
	return errors.New("Name not found")
}

func (dir *Dir) createLocked(ino *Inode, name string) error {
	_, ok := dir.entries[name]
	if ok {
		return errors.New("Name exists")
	}
	dir.entries[name] = ino
	return nil

}

func (dir *Dir) lookup(name string) (*Inode, error) {
	inode, ok := dir.entries[name]
	if ok {
		return inode, nil
	} else {
		return nil, fmt.Errorf("Unknown name %v", name)
	}
}

// Caller must acquire lock?
func (dir *Dir) Len() np.Tlength {
	dir.mu.Lock()
	defer dir.mu.Unlock()
	return dir.lenLocked()
}

func (dir *Dir) namei(path []string, inodes []*Inode) ([]*Inode, []string, error) {
	var inode *Inode
	var err error

	dir.mu.Lock()
	inode, err = dir.lookup(path[0])
	if err != nil {
		log.Printf("dir.Namei %v unknown %v", dir, path)
		dir.mu.Unlock()
		return nil, nil, err
	}
	inodes = append(inodes, inode)
	if inode.IsDir() {
		if len(path) == 1 { // done?
			log.Printf("Namei %v %v -> %v", path, dir, inodes)
			dir.mu.Unlock()
			return inodes, nil, nil
		}
		d := inode.Data.(*Dir)
		dir.mu.Unlock() // for ".."
		return d.namei(path[1:], inodes)
	} else {
		log.Printf("dir.Namei %v %v -> %v %v", path, dir, inodes, path[1:])
		dir.mu.Unlock()
		return inodes, path[1:], nil
	}
}

// for ulambda, cnt is number of directories entries
func (dir *Dir) read(offset np.Toffset, cnt np.Tsize) ([]byte, error) {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	var buf []byte
	if offset >= np.Toffset(dir.lenLocked()) { // passed end of directory
		return buf, nil
	}
	off := np.Toffset(0)
	for n, i := range dir.entries {
		if n == "." {
			continue
		}
		st := *i.Stat()
		st.Name = n
		sz := np.Tsize(npcodec.SizeNp(st))
		if cnt < sz {
			break
		}
		cnt -= sz
		if off >= offset {
			log.Print(st)
			b, err := npcodec.Marshal(st)
			if err != nil {
				return nil, err
			}
			buf = append(buf, b...)
		}
		off += np.Toffset(sz)
	}
	return buf, nil
}

func (dir *Dir) create(inode *Inode, name string) error {
	dir.mu.Lock()
	defer dir.mu.Unlock()

	return dir.createLocked(inode, name)
}

func (dir *Dir) remove(name string) error {
	dir.mu.Lock()
	defer dir.mu.Unlock()
	return dir.removeLocked(name)
}
