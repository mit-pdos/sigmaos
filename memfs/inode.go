package memfs

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	db "ulambda/debug"
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
	inode.mu.Lock()
	defer inode.mu.Unlock()

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

func (inode *Inode) Create(uname string, root *Root, t np.Tperm, name string) (*Inode, error) {
	inode.mu.Lock()
	defer inode.mu.Unlock()

	if IsCurrentDir(name) {
		return nil, errors.New("Cannot create name")
	}
	if inode.IsDir() {
		dir := inode.Data.(*Dir)
		dl, err := permToData(t)
		if err != nil {
			return nil, err
		}
		newi := makeInode(t, root.allocInum(), dl)
		if newi.IsDir() {
			dn := newi.Data.(*Dir)
			dn.init(newi)

		}
		db.DPrintf("%d: create %v in %v -> %v\n", uname, name, inode, newi)
		inode.Mtime = time.Now().Unix()
		return newi, dir.create(newi, name)
	} else {
		return nil, errors.New("Not a directory")
	}
}

func (inode *Inode) Walk(uname string, path []string) ([]*Inode, []string, error) {
	db.DPrintf("%d: Walk %v at %v\n", uname, path, inode)
	inodes := []*Inode{inode}
	if len(path) == 0 {
		return inodes, nil, nil
	}
	dir, ok := inode.Data.(*Dir) // XXX lock
	if !ok {
		return nil, nil, errors.New("Not a directory")
	}
	inodes, rest, err := dir.namei(uname, path, inodes)
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
		return nil, rest, err // XXX was nil?
	}
}

// Lookup a file/directory.  Return parent and inode for file/directory.
// If no parent, return.
func (inode *Inode) LookupPath(uname string, path []string) (*Dir, *Inode, error) {
	inodes, rest, err := inode.Walk(uname, path)
	if err != nil {
		return nil, nil, err
	}
	db.DPrintf("%d: Walk -> %v rest %v err %v\n", uname, inodes, rest, err)
	if len(rest) != 0 {
		return nil, nil, errors.New("Unknown name")
	}
	i := inodes[len(inodes)-1]
	if len(inodes) == 1 {
		return nil, i, nil
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

func (inode *Inode) Remove(uname string, root *Root, path []string) error {
	dir, ino, err := inode.LookupPath(uname, path)
	db.DPrintf("Remove %v dir %v %v %v\n", path, dir, ino, err)
	if err != nil {
		return err
	}
	if len(path) == 0 {
		log.Fatalf("Remove %v dir %v %v %v\n", path, dir, ino, err)
	}
	err = dir.remove(path[len(path)-1])
	if err != nil {
		log.Fatal("Remove error ", err)
	}
	root.freeInum(ino.Inum)
	return nil
}

// XXX open for other types than pipe
func (inode *Inode) Open(mode np.Tmode) error {
	db.DPrintf("inode.Open %v", inode)
	if inode.IsDevice() {
	} else if inode.IsDir() {
	} else if inode.IsSymlink() {
	} else if inode.IsPipe() {
		p := inode.Data.(*Pipe)
		return p.open(mode)
	}
	return nil
}

// XXX open for other types than pipe
func (inode *Inode) Close(mode np.Tmode) error {
	db.DPrintf("inode.Open %v", inode)
	if inode.IsDevice() {
	} else if inode.IsDir() {
	} else if inode.IsSymlink() {
	} else if inode.IsPipe() {
		p := inode.Data.(*Pipe)
		return p.close(mode)
	}
	return nil
}

func (inode *Inode) Write(offset np.Toffset, data []byte) (np.Tsize, error) {
	db.DPrintf("inode.Write %v", inode)
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
	db.DPrintf("inode.Read %v", inode)
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
