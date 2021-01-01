package name

import (
	"errors"
	"log"
)

// Base("/") is "/", so check for "/" too. Base(".") is "." and Dir(".") is
// "." too
func IsCurrentDir(name string) bool {
	return name == "." || name == "/" || name == ""
}

// XXX need mutex
type Root struct {
	inode    *Inode
	nextInum Tinum
}

func MakeRoot() *Root {
	r := Root{}
	r.inode = makeInode(DirT, RootInum, makeDir(RootInum))
	r.nextInum = RootInum + 1
	return &r
}

func (root *Root) RootInode() *Inode {
	return root.inode
}

// XXX bump version # if allocating same inode #
// XXX a better inum allocation plan
func (root *Root) allocInum() Tinum {
	inum := root.nextInum
	root.nextInum += 1
	return inum
}

func (root *Root) freeInum(inum Tinum) {
}

func (root *Root) Namei(dir *Dir, path []string) (*Inode, []string, error) {
	if len(path) == 0 {
		i, err := dir.Lookup(".")
		return i, nil, err
	}
	if IsCurrentDir(path[0]) {
		i, err := dir.Lookup(".")
		return i, path[1:], err
	}
	return dir.Namei(path)
}

func (root *Root) Walk(dir *Dir, path []string) (*Inode, []string, error) {
	log.Printf("name.Walk %v\n", path)
	inode, rest, err := root.Namei(dir, path)
	if err == nil {
		switch inode.Type {
		case MountT:
			// uf := inode.Data.(*fid.Ufid)
			return nil, rest, err
		case SymT:
			// s := inode.Data.(*Symlink)
			return nil, rest, err
		default:
			return inode, rest, err
		}
	} else {
		return nil, nil, err
	}
}

func (root *Root) create(inode *Inode, base string, i interface{}) (*Inode, error) {
	return inode.create(root, FileT, base, i)
}

func (root *Root) mkdir(inode *Inode, base string, i interface{}) (*Inode, error) {
	return inode.create(root, DirT, base, i)
}

func (root *Root) symlink(inode *Inode, base string, i interface{}) (*Inode, error) {
	return inode.create(root, SymT, base, i)
}

func (root *Root) mount(inode *Inode, base string, i interface{}) (*Inode, error) {
	return inode.create(root, MountT, base, i)
}

func (root *Root) mknod(inode *Inode, base string, i interface{}) (*Inode, error) {
	return inode.create(root, DevT, base, i)
}

func (root *Root) pipe(inode *Inode, base string, i interface{}) (*Inode, error) {
	return inode.create(root, PipeT, base, i)
}

func (root *Root) Op(opn string, dir *Inode, path []string,
	op func(*Inode, string, interface{}) (*Inode, error),
	i interface{}) (*Inode, error) {
	log.Printf("name.%v %v %v\n", opn, dir, path)
	// XXX check error
	d := dir.Data.(*Dir)
	if inode, _, err := root.Namei(d, path[:len(path)-1]); err == nil {
		inode, err = op(inode, path[len(path)-1], i)
		if err != nil {
			return inode, err
		}
		log.Printf("name.%v %v %v %v -> %v\n", opn, dir, path, i, inode)
		return inode, nil
	} else {
		return nil, err
	}
}

func (root *Root) Create(dir *Inode, path []string) (*Inode, error) {
	return root.Op("Create", dir, path, root.create, []byte{})
}

func (root *Root) MkDir(dir *Inode, path []string) (*Inode, error) {
	inum := root.allocInum()
	return root.Op("MkDir", dir, path, root.mkdir, makeDir(inum))
}

// func (root *Root) Symlink(dir *Inode, src string, ddir *fid.Ufid, dst string) (*Inode, error) {
// 	return root.Op("Symlink", dir, src, root.symlink, makeSym(ddir, dst))
// }

// func (root *Root) Mount(uf *fid.Ufid, dir *Inode, path string) error {
// 	_, err := root.Op("Mount", dir, path, root.mount, uf)
// 	return err
// }

func (root *Root) MkNod(dir *Inode, path []string, rw Dev) error {
	_, err := root.Op("Mknod", dir, path, root.mknod, rw)
	return err
}

func (root *Root) Pipe(dir *Inode, path []string) error {
	_, err := root.Op("Pipe", dir, path, root.pipe, makePipe())
	return err
}

func (root *Root) Remove(dir *Inode, name string) error {
	log.Printf("name.Remove %v %v\n", dir, name)
	if dir.Type == DirT {
		dir := dir.Data.(*Dir)
		dir.remove(root, name)
	} else {
		errors.New("Base is not a directory")
	}
	return nil
}

func (root *Root) Write(i *Inode, data []byte) (int, error) {
	log.Printf("name.Write %v\n", i)
	// XXX no distinction between DevT and pipeT?
	if i.Type == DevT {
		dev := i.Data.(Dev)
		return dev.Write(i, data)
	} else if i.Type == PipeT {
		pipe := i.Data.(*Pipe)
		return pipe.Write(i, data)
	} else {
		i.Data = data
		return len(data), nil
	}
}

func (root *Root) Read(i *Inode, n int) ([]byte, error) {
	log.Printf("name.Read %v\n", i)
	switch i.Type {
	case DevT:
		dev := i.Data.(Dev)
		return dev.Read(i, n)
	case PipeT:
		pipe := i.Data.(*Pipe)
		return pipe.Read(i, n)
	case FileT:
		return i.Data.([]byte), nil
	case DirT:
		dir := i.Data.(*Dir)
		return []byte(dir.ls(n)), nil
	default:
		return nil, errors.New("Unreadable fid")
	}
}
