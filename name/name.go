package name

import (
	"errors"
	"log"
	"path/filepath"
	"strings"

	"ulambda/fsrpc"
)

type InodeType int

const (
	FileT  InodeType = 1
	DirT   InodeType = 2
	DevT   InodeType = 3
	MountT InodeType = 4
)

type InodeNumber uint64

type Inode struct {
	Type InodeType
	Ino  InodeNumber
	Data interface{}
}

func makeInode(t InodeType, inum InodeNumber, data interface{}) *Inode {
	i := Inode{}
	i.Type = t
	i.Ino = inum
	i.Data = data
	return &i
}

type Dir struct {
	entries map[string]*Inode
}

type Root struct {
	root   *fsrpc.Ufd
	dir    *Dir
	inodes map[InodeNumber]*Inode
	ino    InodeNumber
}

func MakeDir() *Dir {
	d := Dir{}
	d.entries = make(map[string]*Inode)
	return &d
}

func MakeRoot(root *fsrpc.Ufd) *Root {
	d := Root{}
	d.dir = MakeDir()
	d.root = root
	d.inodes = make(map[InodeNumber]*Inode)
	d.ino = 1
	d.inodes[0] = makeInode(DirT, 0, d.dir)
	return &d
}

func (dir *Dir) Lookup(name string) (*Inode, error) {
	inode, ok := dir.entries[name]
	if ok {
		return inode, nil
	} else {
		return nil, errors.New("Unknown name")
	}
}

func (dir *Dir) Namei(path []string) (*Inode, error) {
	var inode *Inode
	var err error
	log.Printf("Namei %v", path)
	inode, err = dir.Lookup(path[0])
	if err != nil {
		return nil, err
	}
	if len(path) == 1 {
		return inode, nil
	}
	if inode.Type == DirT {
		d := inode.Data.(*Dir)
		return d.Namei(path[1:])
	} else {
		return nil, errors.New("Not a directory")
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

func (root *Root) makeUfd(ino InodeNumber) *fsrpc.Ufd {
	ufd := *root.root
	ufd.Fd = fsrpc.Fd(ino)
	return &ufd
}

// XXX walk may run into an intermediate mount
func (root *Root) Walk(path string) (*fsrpc.Ufd, error) {
	if path == "/" || path == "." {
		return root.makeUfd(0), nil
	}
	path = strings.TrimLeft(path, "/")
	inode, err := root.dir.Namei(strings.Split(path, "/"))
	if err != nil {
		return nil, err
	}
	if inode.Type == MountT {
		return inode.Data.(*fsrpc.Ufd), nil
	} else {
		return root.makeUfd(inode.Ino), nil
	}
}

func (root *Root) allocIno() InodeNumber {
	i := root.ino
	root.ino += 1
	return i
}

func (root *Root) Mount(fd *fsrpc.Ufd, path string) error {
	log.Printf("Mount: %v\n", path)
	if path == "/" {
		return errors.New("Cannot mount on root")
	}
	path = strings.TrimLeft(path, "/")
	dirp := filepath.Dir(path)
	base := filepath.Base(path)
	inum := root.allocIno()
	inode := makeInode(MountT, inum, fd)
	if dirp == "." {
		return root.dir.mount(inode, base)
	}
	if inode, err := root.dir.Namei(strings.Split(dirp, "/")); err == nil {
		if inode.Type == DirT {
			d := inode.Data.(*Dir)
			d.mount(inode, base)
			return nil
		} else {
			return errors.New("Base not a directory")
		}
	} else {
		return err
	}
}

func (root *Root) Ls(fd fsrpc.Fd, n int) (string, error) {
	log.Printf("ls %v\n", fd)
	if inode, ok := root.inodes[InodeNumber(fd)]; ok {
		if inode.Type == DirT {
			d := inode.Data.(*Dir)
			return d.ls(n), nil
		} else {
			return "", errors.New("Not a directory")
		}
	} else {
		return "", errors.New("Unknown inode number")
	}
}
