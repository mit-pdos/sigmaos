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

func (root *Root) Create(inode *Inode, name string, perm np.Tperm) (*Inode, error) {
	return inode.create(root, perm, name, []byte{})
}

// If directory recursively remove XXX maybe not
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

func (root *Root) Write(i *Inode, data []byte) (np.Tsize, error) {
	log.Printf("name.Write %v\n", i)
	i.Data = data
	return np.Tsize(len(data)), nil
}

func (root *Root) Read(i *Inode, n np.Tsize) ([]byte, error) {
	log.Printf("name.Read %v\n", i)
	return i.Data.([]byte), nil
}
