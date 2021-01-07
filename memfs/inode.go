package memfs

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

type DataLen interface {
	Len() np.Tlength
}

type Dev interface {
	Write(np.Toffset, []byte) (np.Tsize, error)
	Read(np.Toffset, np.Tsize) ([]byte, error)
	Len() np.Tlength
}

type Inode struct {
	PermT   np.Tperm
	Inum    Tinum
	Version Tversion
	Data    DataLen
}

func makeInode(t np.Tperm, inum Tinum, data DataLen) *Inode {
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
	return np.MakeQid(
		np.Qtype(inode.PermT>>np.QTYPESHIFT),
		np.TQversion(inode.Version),
		np.Tpath(inode.Inum))
}

func (inode *Inode) IsDir() bool {
	return np.IsDir(inode.PermT)
}

func (inode *Inode) IsSymlink() bool {
	return np.IsSymlink(inode.PermT)
}

func (inode *Inode) IsDev() bool {
	return np.IsDevice(inode.PermT)
}

func (inode *Inode) IsPipe() bool {
	return np.IsPipe(inode.PermT)
}

// XXX device
func permToDataLen(t np.Tperm) (DataLen, error) {
	if np.IsDir(t) {
		return makeDir(), nil
	} else if np.IsSymlink(t) {
		return MakeSym(), nil
	} else if np.IsPipe(t) {
		return MakePipe(), nil
	} else if np.IsFile(t) {
		return MakeFile(), nil
	} else {
		return nil, errors.New("Unknown type")
	}
}

func (inode *Inode) Create(root *Root, t np.Tperm, name string) (*Inode, error) {
	if IsCurrentDir(name) {
		return nil, errors.New("Cannot create name")
	}
	if inode.IsDir() {
		d := inode.Data.(*Dir)
		dl, err := permToDataLen(t)
		if err != nil {
			return nil, err
		}
		i := makeInode(t, root.allocInum(), dl)
		if i.IsDir() {
			dir := inode.Data.(*Dir)
			dir.init(i.Inum)
		}
		log.Printf("create %v -> %v\n", name, i)
		return i, d.create(i, name)
	} else {
		return nil, errors.New("Not a directory")
	}
}

func (inode *Inode) Mode() np.Tperm {
	perm := np.Tperm(0777)
	if inode.IsDir() {
		perm |= np.DMDIR
	}
	return perm
}

func (inode *Inode) Stat() *np.Stat {
	stat := &np.Stat{}
	stat.Type = 0 // XXX
	stat.Qid = inode.Qid()
	stat.Mode = inode.Mode()
	stat.Mtime = 0
	stat.Atime = 0
	stat.Length = inode.Data.Len()
	stat.Name = ""
	stat.Uid = "kaashoek"
	stat.Gid = "kaashoek"
	stat.Muid = ""
	return stat
}

func (inode *Inode) Walk(path []string) ([]*Inode, []string, error) {
	log.Printf("Walk %v at %v\n", path, inode)
	if len(path) == 0 {
		return nil, nil, nil
	}
	dir, ok := inode.Data.(*Dir)
	if !ok {
		return nil, nil, errors.New("Not a directory")
	}
	var inodes []*Inode
	inodes, rest, err := dir.Namei(path, inodes)
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

func (inode *Inode) Remove(root *Root, path []string) error {
	start := root.RootInode()
	inodes, rest, err := start.Walk(path)
	if err != nil {
		return err
	}
	if len(rest) != 0 {
		return errors.New("File not found")
	}
	last := len(inodes) - 1
	if inodes[last] != inode {
		log.Fatal("Inode mismatch", inodes, inode)
	}
	parent := start
	if len(inodes) > 1 {
		parent = inodes[last-1]

	}
	dir := parent.Data.(*Dir)
	err = dir.Remove(path[last])
	root.freeInum(inode.Inum)
	if err != nil {
		return err
	}
	return nil
}

func (inode *Inode) Readlink() (string, error) {
	if inode.IsSymlink() {
		s := inode.Data.(*Symlink)
		return s.target, nil
	} else {
		return "", errors.New("Not a symlink")
	}
}

func (inode *Inode) Write(offset np.Toffset, data []byte) (np.Tsize, error) {
	log.Printf("fs.Writei %v\n", inode)
	if inode.IsDev() {
		d := inode.Data.(Dev)
		return d.Write(offset, data)
	} else if inode.IsDir() {
		return 0, errors.New("Cannot write directory")
	} else if inode.IsSymlink() {
		return 0, errors.New("Cannot write symlink")
	} else if inode.IsPipe() {
		p := inode.Data.(*Pipe)
		return p.Write(data)
	} else {
		f := inode.Data.(*File)
		return f.Write(offset, data)
	}
}

func (inode *Inode) Read(offset np.Toffset, n np.Tsize) ([]byte, error) {
	log.Printf("fs.Readi %v\n", inode)
	if inode.IsDev() {
		d := inode.Data.(Dev)
		return d.Read(offset, n)
	} else if inode.IsDir() {
		d := inode.Data.(*Dir)
		return d.Read(offset, n)
	} else if inode.IsSymlink() {
		return nil, errors.New("Cannot read symlink")
	} else if inode.IsPipe() {
		p := inode.Data.(*Pipe)
		return p.Read(n)
	} else { // XXX offset n
		f := inode.Data.(*File)
		return f.Read(offset, n)
	}
}
