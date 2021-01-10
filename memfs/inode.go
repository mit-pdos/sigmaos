package memfs

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	np "ulambda/ninep"
)

type Tinum uint64
type Tversion uint32

const (
	NullInum Tinum = 0
	RootInum Tinum = 1
)

type Data interface {
	Len() np.Tlength
}

type Dev interface {
	Write(np.Toffset, []byte) (np.Tsize, error)
	Read(np.Toffset, np.Tsize) ([]byte, error)
	Len() np.Tlength
}

type Inode struct {
	mu      sync.Mutex
	PermT   np.Tperm
	Inum    Tinum
	Version Tversion
	Mtime   int64
	Data    Data
}

func makeInode(t np.Tperm, inum Tinum, data Data) *Inode {
	i := Inode{}
	i.PermT = t
	i.Inum = inum
	i.Mtime = time.Now().Unix()
	i.Data = data
	return &i
}

func (inode *Inode) String() string {
	str := fmt.Sprintf("Inode %v t 0x%x", inode.Inum,
		inode.PermT>>np.TYPESHIFT)
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

func (inode *Inode) IsDevice() bool {
	return np.IsDevice(inode.PermT)
}

func permToData(t np.Tperm) (Data, error) {
	if np.IsDir(t) {
		return makeDir(), nil
	} else if np.IsSymlink(t) {
		return MakeSym(), nil
	} else if np.IsPipe(t) {
		return MakePipe(), nil
	} else if np.IsDevice(t) {
		return nil, nil
	} else if np.IsFile(t) {
		return MakeFile(), nil
	} else {
		return nil, errors.New("Unknown type")
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
	stat.Mtime = uint32(inode.Mtime)
	stat.Atime = 0
	stat.Length = inode.Data.Len()
	stat.Name = ""
	stat.Uid = "kaashoek"
	stat.Gid = "kaashoek"
	stat.Muid = ""
	return stat
}

func (inode *Inode) Create(root *Root, t np.Tperm, name string) (*Inode, error) {
	if IsCurrentDir(name) {
		return nil, errors.New("Cannot create name")
	}
	if inode.IsDir() {
		d := inode.Data.(*Dir)
		dl, err := permToData(t)
		if err != nil {
			return nil, err
		}
		i := makeInode(t, root.allocInum(), dl)
		if i.IsDir() {
			dir := inode.Data.(*Dir)
			dir.init(i)
		}
		log.Printf("create %v -> %v\n", name, i)
		inode.Mtime = time.Now().Unix()
		return i, d.create(i, name)
	} else {
		return nil, errors.New("Not a directory")
	}
}

func (inode *Inode) Walk(path []string) ([]*Inode, []string, error) {
	log.Printf("Walk %v at %v\n", path, inode)
	inodes := []*Inode{inode}
	if len(path) == 0 {
		return inodes, nil, nil
	}
	dir, ok := inode.Data.(*Dir) // XXX lock
	if !ok {
		return nil, nil, errors.New("Not a directory")
	}
	inodes, rest, err := dir.namei(path, inodes)
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

// Lookup a directory or file. If file, return parent dir and inode
// for file.  If directory, return it
func (inode *Inode) LookupPath(path []string) (*Dir, *Inode, error) {
	inodes, rest, err := inode.Walk(path)
	if err != nil {
		return nil, nil, err
	}
	if len(rest) != 0 {
		return nil, nil, errors.New("Unknown name")
	}
	i := inodes[len(inodes)-1]
	if i.IsDir() {
		return i.Data.(*Dir), nil, nil
	} else {
		// there must be a parent
		di := inodes[len(inodes)-2]
		dir, ok := di.Data.(*Dir)
		if !ok {
			log.Fatal("Lookup: cast error")
		}
		return dir, inodes[len(inodes)-1], nil
	}
}

func (inode *Inode) Remove(root *Root, path []string) error {
	dir, ino, err := inode.LookupPath(path)
	if err != nil {
		return err
	}
	err = dir.remove(path[len(path)-1])
	if err != nil {
		log.Fatal("Remove error ", err)
	}
	root.freeInum(ino.Inum)
	return nil
}

func (inode *Inode) Write(offset np.Toffset, data []byte) (np.Tsize, error) {
	log.Print("fs.Writei ", inode)
	var sz np.Tsize
	var err error
	if inode.IsDevice() {
		d := inode.Data.(Dev)
		sz, err = d.Write(offset, data)
	} else if inode.IsDir() {
		return 0, errors.New("Cannot write directory")
	} else if inode.IsSymlink() {
		s := inode.Data.(*Symlink)
		sz, err = s.write(data)
	} else if inode.IsPipe() {
		p := inode.Data.(*Pipe)
		sz, err = p.write(data)
	} else {
		f := inode.Data.(*File)
		sz, err = f.write(offset, data)
	}
	if err != nil {
		inode.Mtime = time.Now().Unix()
	}
	return sz, err
}

func (inode *Inode) Read(offset np.Toffset, n np.Tsize) ([]byte, error) {
	log.Print("fs.Readi ", inode)
	if inode.IsDevice() {
		d := inode.Data.(Dev)
		return d.Read(offset, n)
	} else if inode.IsDir() {
		d := inode.Data.(*Dir)
		return d.read(offset, n)
	} else if inode.IsSymlink() {
		s := inode.Data.(*Symlink)
		return s.read(n)
	} else if inode.IsPipe() {
		p := inode.Data.(*Pipe)
		return p.read(n)
	} else {
		f := inode.Data.(*File)
		return f.read(offset, n)
	}
}
