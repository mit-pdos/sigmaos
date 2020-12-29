package name

import (
	"errors"
	// "log"

	"ulambda/fid"
)

type Dev interface {
	Write(fid.Fid, []byte) (int, error)
	Read(fid.Fid, int) ([]byte, error)
}

type Inode struct {
	Type fid.IType
	Fid  fid.Fid
	Data interface{}
}

func makeInode(t fid.IType, fid fid.Fid, data interface{}) *Inode {
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
	if inode.Type == fid.DirT {
		d := inode.Data.(*Dir)
		return d.Lookup(name)
	} else {
		return nil, errors.New("Base not a directory")
	}
}

func (inode *Inode) create(root *Root, t fid.IType, name string, data interface{}) (*Inode, error) {
	if IsCurrentDir(name) {
		return nil, errors.New("Cannot create name")
	}
	if inode.Type == fid.DirT {
		d := inode.Data.(*Dir)
		i := makeInode(t, root.allocInum(), data)
		return i, d.create(i, name)
	} else {
		return nil, errors.New("Base not a directory")
	}
}
