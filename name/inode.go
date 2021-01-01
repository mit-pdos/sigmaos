package name

import (
	"errors"
)

type IType int
type Tinum uint64
type Tversion uint32

const (
	NullInum Tinum = 0
	RootInum Tinum = 1
)

const (
	FileT  IType = 1
	DirT   IType = 2
	DevT   IType = 3
	MountT IType = 4
	PipeT  IType = 5
	SymT   IType = 6
)

type Dev interface {
	Write(*Inode, []byte) (int, error)
	Read(*Inode, int) ([]byte, error)
}

type Inode struct {
	Type    IType
	Inum    Tinum
	Version Tversion
	Data    interface{}
}

func makeInode(t IType, inum Tinum, data interface{}) *Inode {
	i := Inode{}
	i.Type = t
	i.Inum = inum
	i.Data = data
	return &i
}

func (inode *Inode) lookup(name string) (*Inode, error) {
	if IsCurrentDir(name) {
		return inode, nil
	}
	if inode.Type == DirT {
		d := inode.Data.(*Dir)
		return d.Lookup(name)
	} else {
		return nil, errors.New("Base not a directory")
	}
}

func (inode *Inode) create(root *Root, t IType, name string, data interface{}) (*Inode, error) {
	if IsCurrentDir(name) {
		return nil, errors.New("Cannot create name")
	}
	if inode.Type == DirT {
		d := inode.Data.(*Dir)
		i := makeInode(t, root.allocInum(), data)
		return i, d.create(i, name)
	} else {
		return nil, errors.New("Base not a directory")
	}
}
