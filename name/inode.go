package name

import (
	"errors"
	// "log"

	"ulambda/fsrpc"
)

type IType int

const (
	FileT  IType = 1
	DirT   IType = 2
	DevT   IType = 3
	MountT IType = 4
	PipeT  IType = 5
)

type Dev interface {
	Write(fsrpc.Fid, []byte) (int, error)
	Read(fsrpc.Fid, int) ([]byte, error)
}

type Inode struct {
	Type IType
	Fid  fsrpc.Fid
	Data interface{}
}

func makeInode(t IType, fid fsrpc.Fid, data interface{}) *Inode {
	i := Inode{}
	i.Type = t
	i.Fid = fid
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
		return d.create(i, name)
	} else {
		return nil, errors.New("Base not a directory")
	}
}
