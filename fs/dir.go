package fs

import (
	"errors"
	"fmt"
	"log"

	np "ulambda/ninep"
	"ulambda/npcodec"
)

// Base("/") is "/", so check for "/" too. Base(".") is "." and Dir(".") is
// "." too
func IsCurrentDir(name string) bool {
	return name == "." || name == "/" || name == ""
}

type Dir struct {
	entries map[string]*Inode
	entrysz uint32
}

func makeDir() *Dir {
	d := &Dir{}
	d.entrysz = npcodec.SizeNp(np.Stat{})
	d.entries = make(map[string]*Inode)

	return d
}

func (dir *Dir) init(inum Tinum) {
	dir.entries["."] = makeInode(np.DMDIR, inum, dir)
}

func (dir *Dir) Len() np.Tlength {
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

func (dir *Dir) Lookup(name string) (*Inode, error) {
	inode, ok := dir.entries[name]
	if ok {
		return inode, nil
	} else {
		return nil, fmt.Errorf("Unknown name %v", name)

	}
}

func (dir *Dir) Namei(path []string, inodes []*Inode) ([]*Inode, []string, error) {
	var inode *Inode
	var err error

	inode, err = dir.Lookup(path[0])
	if err != nil {
		log.Printf("dir.Namei %v unknown %v", dir, path)
		return nil, nil, err
	}
	inodes = append(inodes, inode)
	if inode.IsDir() {
		if len(path) == 1 { // done?
			log.Printf("Namei %v %v -> %v", path, dir, inodes)
			return inodes, nil, nil
		}
		d := inode.Data.(*Dir)
		return d.Namei(path[1:], inodes)
	} else {
		log.Printf("dir.Namei %v %v -> %v %v", path, dir, inodes, path[1:])
		return inodes, path[1:], nil
	}
}

func (dir *Dir) Read(offset np.Toffset, n np.Tsize) ([]byte, error) {
	buf := []byte{}
	if offset == 0 {
		for n, i := range dir.entries {
			st := *i.Stat()
			st.Name = n
			b, err := npcodec.Marshal(st)
			if err != nil {
				return nil, err
			}
			buf = append(buf, b...)
		}
	}
	return buf, nil
}

func (dir *Dir) create(inode *Inode, name string) error {
	_, ok := dir.entries[name]
	if ok {
		return errors.New("Name exists")
	}
	dir.entries[name] = inode
	return nil
}

func (dir *Dir) Remove(name string) error {
	_, ok := dir.entries[name]
	if ok {
		delete(dir.entries, name)
		return nil
	}
	return errors.New("Name not found")
}
