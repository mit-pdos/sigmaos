package fs

import (
	"errors"
	"fmt"
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

func (inode *Inode) String() string {
	str := fmt.Sprintf("Inode %v t 0x%x data %v {}", inode.Inum, inode.PermT>>np.TYPESHIFT,
		inode.Data)
	return str
}

func (inode *Inode) Qid() np.Tqid {
	return np.MakeQid(np.Qtype(inode.PermT>>np.QTYPESHIFT), np.TQversion(inode.Version), np.Tpath(inode.Inum))
}

func (inode *Inode) isDir() bool {
	return inode.PermT&np.DMDIR == np.DMDIR
}

func (inode *Inode) isSymlink() bool {
	return inode.PermT&np.DMSYMLINK == np.DMSYMLINK
}

func (inode *Inode) isDev() bool {
	return inode.PermT&np.DMDEVICE == np.DMDEVICE
}

func (inode *Inode) isPipe() bool {
	return inode.PermT&np.DMNAMEDPIPE == np.DMNAMEDPIPE
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
		log.Printf("create %v -> %v\n", name, i)
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
	log.Printf("fs.Writei %v\n", inode)
	if inode.isDev() {
		d := inode.Data.(Dev)
		return d.Write(data)
	} else if inode.isDir() {
		return 0, errors.New("Cannot write directory")
	} else if inode.isSymlink() {
		return 0, errors.New("Cannot write symlink")
	} else if inode.isPipe() {
		p := inode.Data.(*Pipe)
		return p.Write(data)
	} else {
		inode.Data = data
		return np.Tsize(len(data)), nil
	}
}

func (inode *Inode) Read(n np.Tsize) ([]byte, error) {
	log.Printf("fs.Readi %v\n", inode)
	if inode.isDev() {
		d := inode.Data.(Dev)
		return d.Read(n)
	} else if inode.isDir() {
		return nil, errors.New("Cannot read directory")
	} else if inode.isSymlink() {
		return nil, errors.New("Cannot read symlink")
	} else if inode.isPipe() {
		p := inode.Data.(*Pipe)
		return p.Read(n)
	} else {
		d := inode.Data.([]byte)
		return d, nil
	}
}
