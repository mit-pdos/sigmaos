package name

import (
	"errors"
	"log"

	np "ulambda/ninep"
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
	r.inode = makeInode(np.DMDIR, RootInum, makeDir(RootInum))
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

func (root *Root) Namei(dir *Dir, path []string,
	inodes []*Inode) ([]*Inode, []string, error) {
	if len(path) == 0 {
		i, err := dir.Lookup(".")
		return append(inodes, i), nil, err
	}
	if IsCurrentDir(path[0]) {
		i, err := dir.Lookup(".")
		return append(inodes, i), path[1:], err
	}
	return dir.Namei(path, inodes)
}

func (root *Root) Walk(dir *Dir, path []string) ([]*Inode, []string, error) {
	log.Printf("name.Walk %v\n", path)
	var inodes []*Inode
	inodes, rest, err := root.Namei(dir, path, inodes)
	if err == nil {
		return inodes, rest, err
		// switch inodes[len(inodes)-1].PermT {
		// case MountT:
		// 	// uf := inode.Data.(*fid.Ufid)
		// 	return nil, rest, err
		// case SymT:
		// 	// s := inode.Data.(*Symlink)
		// 	return nil, rest, err
		// default:
	} else {
		return nil, nil, err
	}
}

// func (root *Root) symlink(inode *Inode, base string, i interface{}) (*Inode, error) {
// 	return inode.create(root, SymT, base, i)
// }

// func (root *Root) mount(inode *Inode, base string, i interface{}) (*Inode, error) {
// 	return inode.create(root, MountT, base, i)
// }

// func (root *Root) mknod(inode *Inode, base string, i interface{}) (*Inode, error) {
// 	return inode.create(root, DevT, base, i)
// }

// func (root *Root) pipe(inode *Inode, base string, i interface{}) (*Inode, error) {
// 	return inode.create(root, PipeT, base, i)
// }

func (root *Root) Create(inode *Inode, name string, perm np.Tperm) (*Inode, error) {
	return inode.create(root, perm, name, []byte{})
}

// func (root *Root) Symlink(dir *Inode, src string, ddir *fid.Ufid, dst string) (*Inode, error) {
// 	return root.Op("Symlink", dir, src, root.symlink, makeSym(ddir, dst))
// }

// func (root *Root) Mount(uf *fid.Ufid, dir *Inode, path string) error {
// 	_, err := root.Op("Mount", dir, path, root.mount, uf)
// 	return err
// }

// func (root *Root) MkNod(dir *Inode, name string, rw Dev) error {
// 	_, err := root.Op("Mknod", dir, name, root.mknod, rw)
// 	return err
// }

// func (root *Root) Pipe(dir *Inode, name string) error {
// 	_, err := root.Op("Pipe", dir, name, root.pipe, makePipe())
// 	return err
// }

func (root *Root) Remove(dir *Inode, name string) error {
	log.Printf("name.Remove %v %v\n", dir, name)
	if dir.isDir() {
		dir := dir.Data.(*Dir)
		dir.remove(root, name)
	} else {
		errors.New("Base is not a directory")
	}
	return nil
}

func (root *Root) Write(i *Inode, data []byte) (int, error) {
	log.Printf("name.Write %v\n", i)
	i.Data = data
	return len(data), nil
}

func (root *Root) Read(i *Inode, n int) ([]byte, error) {
	log.Printf("name.Read %v\n", i)
	return i.Data.([]byte), nil
}
