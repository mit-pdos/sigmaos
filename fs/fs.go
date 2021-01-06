package fs

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
	r.inode = makeInode(np.DMDIR, RootInum, makeDir())
	r.nextInum = RootInum + 1
	dir := r.inode.Data.(*Dir)
	dir.init(r.inode.Inum)
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

func (root *Root) Namei(dir *Dir, path []string, inodes []*Inode) ([]*Inode, []string, error) {
	i, err := dir.Lookup(path[0])
	if err != nil {
		return inodes, path, err
	}
	inodes = append(inodes, i)
	if len(path) == 1 {
		return inodes, nil, err
	}
	return dir.Namei(path[1:], inodes)
}

func (root *Root) Walk(inode *Inode, path []string) ([]*Inode, []string, error) {
	log.Printf("fs.Walk %v at %v\n", path, inode)
	if len(path) == 0 {
		return nil, nil, nil
	}
	dir, ok := inode.Data.(*Dir)
	if !ok {
		return nil, nil, errors.New("Not a directory")
	}
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

func (root *Root) Create(inode *Inode, name string, perm np.Tperm) (*Inode, error) {
	log.Printf("fs.Create %v %v\n", inode, name)
	return inode.create(root, perm, name, []byte{})
}

func (root *Root) Mkdir(inode *Inode, name string) (*Inode, error) {
	log.Printf("fs.Mkdir %v %v\n", inode, name)
	inode, err := inode.create(root, np.DMDIR, name, makeDir())
	if err != nil {
		return nil, err
	}
	dir := inode.Data.(*Dir)
	dir.init(inode.Inum)
	return inode, nil
}

func (root *Root) Mkpipe(inode *Inode, name string) (*Inode, error) {
	return inode.create(root, np.DMNAMEDPIPE, name, makePipe())
}

func (root *Root) Symlink(inode *Inode, name string, target string) (*Inode, error) {
	return inode.create(root, np.DMSYMLINK, name, makeSym(target))
}

func (root *Root) MkNod(inode *Inode, name string, i interface{}) (*Inode, error) {
	return inode.create(root, np.DMDEVICE, name, i)
}

// If directory recursively remove XXX maybe not
func (root *Root) Remove(dir *Inode, name string) error {
	log.Printf("fs.Remove %v %v\n", dir, name)
	if dir.isDir() {
		dir := dir.Data.(*Dir)
		dir.remove(root, name)
	} else {

	}
	return nil
}
