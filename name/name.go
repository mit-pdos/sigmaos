package name

import (
	"errors"
	"log"
	"path/filepath"
	"strings"

	"ulambda/fsrpc"
)

type InodeType int
type InodeNumber uint64

const (
	RootInum InodeNumber = 1

	FileT  InodeType = 1
	DirT   InodeType = 2
	DevT   InodeType = 3
	MountT InodeType = 4
)

type Inode struct {
	Type InodeType
	Inum InodeNumber
	Data interface{}
}

func makeInode(t InodeType, inum InodeNumber, data interface{}) *Inode {
	i := Inode{}
	i.Type = t
	i.Inum = inum
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
	inum   InodeNumber
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
	d.inum = RootInum
	d.inodes[RootInum] = makeInode(DirT, RootInum, d.dir)
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

func (dir *Dir) Namei(path []string) (*Inode, []string, error) {
	var inode *Inode
	var err error
	inode, err = dir.Lookup(path[0])
	if err != nil {
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

func (dir *Dir) create(inode *Inode, name string) error {
	_, ok := dir.entries[name]
	if ok {
		return errors.New("Name exists")
	}
	dir.entries[name] = inode
	return nil
}

func (root *Root) Myroot() *fsrpc.Ufd {
	return root.root
}

func (root *Root) makeUfd(inum InodeNumber) *fsrpc.Ufd {
	ufd := *root.root
	ufd.Fd = fsrpc.Fd(inum)
	return &ufd
}

func (root *Root) Namei(path []string) (*Inode, []string, error) {
	if path[0] == "." {
		return root.inodes[RootInum], path[1:], nil
	}
	return root.dir.Namei(path)
}

func (root *Root) Walk(path string) (*fsrpc.Ufd, string, error) {
	log.Printf("Walk %v\n", path)
	if path == "/" || path == "." {
		return root.makeUfd(RootInum), "", nil
	}
	path = strings.TrimLeft(path, "/")
	inode, rest, err := root.Namei(strings.Split(path, "/"))
	if err == nil {
		if inode.Type == MountT {
			ufd := inode.Data.(*fsrpc.Ufd)
			return ufd, strings.Join(rest, "/"), err
		} else {
			return root.makeUfd(inode.Inum), strings.Join(rest, "/"), err
		}
	} else {
		return nil, "", err
	}
}

func (root *Root) Mount(ufd *fsrpc.Ufd, path string) error {
	if path == "/" {
		return errors.New("Cannot mount on root")
	}
	path = strings.TrimLeft(path, "/")
	dirp := filepath.Dir(path)
	base := filepath.Base(path)
	log.Printf("Mount %v at (%v,%v)\n", ufd, dirp, base)
	i := makeInode(MountT, 0, ufd)
	if inode, _, err := root.Namei(strings.Split(dirp, "/")); err == nil {
		if inode.Type == DirT {
			d := inode.Data.(*Dir)
			d.mount(i, base)
			return nil
		} else {
			return errors.New("Base not a directory")
		}
	} else {
		return err
	}
}

func (root *Root) Ls(fd fsrpc.Fd, n int) (string, error) {
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

func (root *Root) Create(path string, inum InodeNumber, inodeData interface{}) error {
	dirp := filepath.Dir(path)
	base := filepath.Base(path)
	log.Printf("Create %v(%v,%v) %v\n", path, dirp, base, inum)
	if path == "/" || path == "." {
		return errors.New("Cannot create root and .")
	}
	path = strings.TrimLeft(path, "/")
	i := makeInode(FileT, inum, inodeData)
	if inode, _, err := root.Namei(strings.Split(dirp, "/")); err == nil {
		log.Printf("inode %v", inode)
		if inode.Type == DirT {
			d := inode.Data.(*Dir)
			d.create(i, base)
			root.inodes[inum] = i
			return nil
		} else {
			return errors.New("Base not a directory")
		}
	} else {
		return err
	}
}

func (root *Root) Fd2Inode(fd fsrpc.Fd) (*Inode, error) {
	if inode, ok := root.inodes[InodeNumber(fd)]; ok {
		return inode, nil
	} else {
		return nil, errors.New("Unknown fd")
	}
}

func (root *Root) Open(ufd *fsrpc.Ufd) (*Inode, error) {
	return root.Fd2Inode(ufd.Fd)
}
