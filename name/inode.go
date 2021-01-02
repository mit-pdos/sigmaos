package name

import (
	"errors"

	np "ulambda/ninep"
)

type Tinum uint64
type Tversion uint32

const (
	NullInum Tinum = 0
	RootInum Tinum = 1
)

type Dev interface {
	Write(*Inode, []byte) (int, error)
	Read(*Inode, int) ([]byte, error)
}

type Inode struct {
	PermT   np.Tperm
	Inum    Tinum
	Version Tversion
	Data    interface{}
}

func makeInode(t np.Tperm, inum Tinum, data interface{}) *Inode {
	i := Inode{}
	i.PermT = t
	i.Inum = inum
	i.Data = data
	return &i
}

func (inode *Inode) isDir() bool {
	return inode.PermT&np.DMDIR == np.DMDIR
}

func (inode *Inode) lookup(name string) (*Inode, error) {
	if IsCurrentDir(name) {
		return inode, nil
	}
	if inode.isDir() {
		d := inode.Data.(*Dir)
		return d.Lookup(name)
	} else {
		return nil, errors.New("Base not a directory")
	}
}

func (inode *Inode) create(root *Root, t np.Tperm, name string, data interface{}) (*Inode, error) {
	if IsCurrentDir(name) {
		return nil, errors.New("Cannot create name")
	}
	if inode.isDir() {
		d := inode.Data.(*Dir)
		i := makeInode(t, root.allocInum(), data)
		return i, d.create(i, name)
	} else {
		return nil, errors.New("Base not a directory")
	}
}
