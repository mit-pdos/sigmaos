package fs

import (
	"log"

	np "ulambda/ninep"
)

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

func (root *Root) Mkdir(inode *Inode, name string) (*Inode, error) {
	log.Printf("fs.Mkdir %v %v\n", inode, name)
	inode, err := inode.Create(root, np.DMDIR, name, makeDir())
	if err != nil {
		return nil, err
	}
	dir := inode.Data.(*Dir)
	dir.init(inode.Inum)
	return inode, nil
}

func (root *Root) MkNod(inode *Inode, name string, i DataLen) (*Inode, error) {
	return inode.Create(root, np.DMDEVICE, name, i)
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
