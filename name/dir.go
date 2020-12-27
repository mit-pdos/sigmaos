package name

import (
	"errors"
	"fmt"
	"log"
	"strings"
)

type Dir struct {
	entries map[string]*Inode
}

func makeDir() *Dir {
	d := Dir{}
	d.entries = make(map[string]*Inode)
	return &d
}

func (dir *Dir) Lookup(name string) (*Inode, error) {
	inode, ok := dir.entries[name]
	if ok {
		return inode, nil
	} else {
		return nil, fmt.Errorf("Unknown name %v", name)

	}
}

func (dir *Dir) Namei(path []string) (*Inode, []string, error) {
	var inode *Inode
	var err error

	inode, err = dir.Lookup(path[0])
	if err != nil {
		log.Printf("Namei %v non existing", path)
		return nil, nil, err
	}
	if len(path) == 1 { // done?
		log.Printf("Namei %v %v -> %v", path, dir, inode)
		return inode, nil, nil
	}
	if inode.Type == DirT {
		d := inode.Data.(*Dir)
		return d.Namei(path[1:])
	} else if inode.Type == MountT {
		log.Printf("Namei %v %v -> %v %v", path, dir, inode, path[1:])
		return inode, path[1:], nil
	} else {
		return nil, path[1:], errors.New("Not a directory")
	}
}

func (dir *Dir) ls(n int) string {
	names := make([]string, 0, len(dir.entries))
	for k, _ := range dir.entries {
		names = append(names, k)
	}
	return strings.Join(names, " ") + "\n"
}

func (dir *Dir) mount(inode *Inode, name string) error {
	_, ok := dir.entries[name]
	if ok {
		return errors.New("Name exists")
	}
	dir.entries[name] = inode
	return nil
}

func (dir *Dir) create(inode *Inode, name string) (*Inode, error) {
	_, ok := dir.entries[name]
	if ok {
		return nil, errors.New("Name exists")
	}
	dir.entries[name] = inode
	return inode, nil
}
