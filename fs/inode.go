package fs

import (
	"errors"
	"log"

	np "ulambda/ninep"
)

type Tinum uint64
type Tversion uint32

const (
	NullInum Tinum = 0
	RootInum Tinum = 1
)

type Dev interface {
	Write([]byte) (np.Tsize, error)
	Read(np.Tsize) ([]byte, error)
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

func (inode *Inode) Qid() np.Tqid {
	return np.MakeQid(np.Qtype(inode.PermT>>24), np.TQversion(inode.Version), np.Tpath(inode.Inum))
}

func (inode *Inode) isDir() bool {
	return inode.PermT&np.DMDIR == np.DMDIR
}

func (inode *Inode) isSymlink() bool {
	return inode.PermT&np.DMSYMLINK == np.DMSYMLINK
}

func (inode *Inode) isDevice() bool {
	return inode.PermT&np.DMDEVICE == np.DMDEVICE
}

func (inode *Inode) lookup(name string) (*Inode, error) {
	if IsCurrentDir(name) {
		return inode, nil
	}
	if inode.isDir() {
		d := inode.Data.(*Dir)
		return d.Lookup(name)
	} else {
		return nil, errors.New("Not a directory")
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
		return nil, errors.New("Not a directory")
	}
}

func (inode *Inode) Readlink() (string, error) {
	if inode.isSymlink() {
		s := inode.Data.(*Symlink)
		return s.target, nil
	} else {
		return "", errors.New("Not a symlink")
	}
}

func (inode *Inode) Write(data []byte) (np.Tsize, error) {
	log.Printf("Writei %v\n", inode)
	if inode.isDevice() {
		d := inode.Data.(Dev)
		return d.Write(data)
	} else {
		inode.Data = data
		return np.Tsize(len(data)), nil
	}
}

func (inode *Inode) Read(n np.Tsize) ([]byte, error) {
	log.Printf("Readi %v\n", inode)
	if inode.isDevice() {
		d := inode.Data.(Dev)
		return d.Read(n)
	} else {
		d := inode.Data.([]byte)
		return d, nil
	}
}
