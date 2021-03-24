package memfs

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
	"unsafe"

	db "ulambda/debug"
	np "ulambda/ninep"
	npo "ulambda/npobjsrv"
)

type Tinum uint64
type Tversion uint32

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
	permT   np.Tperm
	version np.TQversion
	Mtime   int64
	Data    Data
	parent  *Inode
	owner   string
}

func makeInode(owner string, t np.Tperm, data Data, p *Inode) *Inode {
	i := Inode{}
	i.permT = t
	i.Mtime = time.Now().Unix()
	i.Data = data
	i.parent = p
	i.owner = owner
	return &i
}

func MkRootInode() *Inode {
	return makeInode("", np.DMDIR, makeDir(), nil)
}

func (inode *Inode) String() string {
	str := fmt.Sprintf("Inode %p t 0x%x", inode,
		inode.permT>>np.TYPESHIFT)
	return str
}

func (inode *Inode) Qid() np.Tqid {
	id := uintptr(unsafe.Pointer(inode))

	return np.MakeQid(
		np.Qtype(inode.permT>>np.QTYPESHIFT),
		np.TQversion(inode.version),
		np.Tpath(uint64(id)))
}

func (inode *Inode) Perm() np.Tperm {
	return inode.permT
}

func (inode *Inode) Version() np.TQversion {
	return inode.version
}

func (inode *Inode) Size() np.Tlength {
	if inode.IsDir() {
		d := inode.Data.(*Dir)
		return d.Len()
	}
	return inode.Data.Len()
}

func (inode *Inode) IsDir() bool {
	return inode.permT.IsDir()
}

func (inode *Inode) IsSymlink() bool {
	return inode.permT.IsSymlink()
}

func (inode *Inode) IsPipe() bool {
	return inode.permT.IsPipe()
}

func (inode *Inode) IsDevice() bool {
	return inode.permT.IsDevice()
}

func permToData(t np.Tperm) (Data, error) {
	if t.IsDir() {
		return makeDir(), nil
	} else if t.IsSymlink() {
		return MakeSym(), nil
	} else if t.IsPipe() {
		return MakePipe(), nil
	} else if t.IsDevice() {
		return nil, nil
	} else if t.IsFile() {
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

func (inode *Inode) stat() (*np.Stat, error) {
	stat := &np.Stat{}
	stat.Type = 0 // XXX
	stat.Qid = inode.Qid()
	stat.Mode = inode.Mode()
	stat.Mtime = uint32(inode.Mtime)
	stat.Atime = 0
	stat.Length = inode.Data.Len()
	stat.Name = ""
	stat.Uid = inode.owner
	stat.Gid = inode.owner
	stat.Muid = ""
	return stat, nil
}

func (inode *Inode) Stat(Ctx *npo.Ctx) (*np.Stat, error) {
	inode.mu.Lock()
	defer inode.mu.Unlock()
	return inode.stat()
}

func (inode *Inode) Create(ctx *npo.
	Ctx, name string, t np.Tperm, m np.Tmode) (npo.NpObj, error) {
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
		newi := makeInode(ctx.Uname(), t, dl, inode)
		if newi.IsDir() {
			dn := newi.Data.(*Dir)
			dn.init(newi)

		}
		db.DLPrintf("MEMFS", "Create %v in %v -> %v\n", name, inode, newi)
		inode.Mtime = time.Now().Unix()
		return newi, dir.create(newi, name)
	} else {
		return nil, errors.New("Not a directory")
	}
}

func (inode *Inode) Lookup(ctx *npo.Ctx, path []string) ([]npo.NpObj, []string, error) {
	db.DLPrintf("MEMFS", "%v: Lookup %v %v\n", ctx, inode, path)
	inodes := []npo.NpObj{}
	if len(path) == 0 {
		return nil, nil, nil
	}
	dir, ok := inode.Data.(*Dir) // XXX lock
	if !ok {
		return nil, nil, errors.New("Not a directory")
	}
	db.DLPrintf("MEMFS", "lookup: %v\n", path)
	inodes, rest, err := dir.namei(ctx, path, inodes)
	if err == nil {
		return inodes, rest, err
	} else {
		return nil, rest, err // XXX was nil?
	}
}

func (inode *Inode) Remove(ctx *npo.Ctx, n string) error {
	if inode.parent == nil {
		return errors.New("Cannot remove root directory")
	}
	if !inode.parent.IsDir() {
		return errors.New("Parent not a directory")
	}
	dir := inode.parent.Data.(*Dir)
	dir.mu.Lock()
	defer dir.mu.Unlock()

	i1, err := dir.lookupLocked(n)
	if err != nil {
		return fmt.Errorf("Unknown name %v", n)
	}
	i1.version += 1
	err = dir.removeLocked(n)
	if err != nil {
		log.Fatalf("Remove: error %v\n", n)
	}
	return nil
}

// XXX open for other types than pipe
func (inode *Inode) Open(ctx *npo.Ctx, mode np.Tmode) error {
	db.DLPrintf("MEMFS", "inode.Open %v", inode)
	if inode.IsPipe() {
		p := inode.Data.(*Pipe)
		return p.open(ctx, mode)
	}
	return nil
}

// XXX open for other types than pipe
func (inode *Inode) Close(ctx *npo.Ctx, mode np.Tmode) error {
	db.DLPrintf("MEMFS", "inode.Open %v", inode)
	if inode.IsDevice() {
	} else if inode.IsDir() {
	} else if inode.IsSymlink() {
	} else if inode.IsPipe() {
		p := inode.Data.(*Pipe)
		return p.close(ctx, mode)
	}
	return nil
}

func (inode *Inode) WriteFile(ctx *npo.Ctx, offset np.Toffset, data []byte) (np.Tsize, error) {
	db.DLPrintf("MEMFS", "inode.Write %v", inode)
	var sz np.Tsize
	var err error
	if inode.IsDevice() {
		d := inode.Data.(Dev)
		sz, err = d.Write(offset, data)
	} else if inode.IsSymlink() {
		s := inode.Data.(*Symlink)
		sz, err = s.write(data)
	} else if inode.IsPipe() {
		p := inode.Data.(*Pipe)
		sz, err = p.write(ctx, data)
	} else {
		f := inode.Data.(*File)
		sz, err = f.write(offset, data)
	}
	if err != nil {
		inode.Mtime = time.Now().Unix()
	}
	return sz, err
}

func (inode *Inode) ReadDir(ctx *npo.Ctx, offset np.Toffset, n np.Tsize) ([]*np.Stat, error) {
	d := inode.Data.(*Dir)
	return d.read(offset, n)
}

func (inode *Inode) WriteDir(ctx *npo.Ctx, offset np.Toffset, b []byte) (np.Tsize, error) {
	return 0, errors.New("Cannot write directory")
}

func (inode *Inode) ReadFile(ctx *npo.Ctx, offset np.Toffset, n np.Tsize) ([]byte, error) {
	db.DLPrintf("MEMFS", "inode.Read %v", inode)
	if inode.IsDevice() {
		d := inode.Data.(Dev)
		return d.Read(offset, n)
	} else if inode.IsSymlink() {
		s := inode.Data.(*Symlink)
		return s.read(offset, n)
	} else if inode.IsPipe() {
		p := inode.Data.(*Pipe)
		return p.read(ctx, n)
	} else {
		f := inode.Data.(*File)
		return f.read(offset, n)
	}
}

func (inode *Inode) Rename(ctx *npo.Ctx, from, to string) error {
	if inode.parent == nil {
		return errors.New("Cannot remove root directory")
	}
	if !inode.parent.IsDir() {
		return errors.New("Parent not a directory")
	}
	dir := inode.parent.Data.(*Dir)
	dir.mu.Lock()
	defer dir.mu.Unlock()

	db.DLPrintf("MEMFS", "%v: Rename %v -> %v\n", dir, from, to)
	ino, err := dir.lookupLocked(from)
	if err != nil {
		return fmt.Errorf("Unknown name %v", from)
	}
	err = dir.removeLocked(from)
	if err != nil {
		log.Fatalf("Rename: remove failed %v %v\n", from, err)
	}
	_, err = dir.lookupLocked(to)
	if err == nil { // i is valid
		// XXX 9p: it is an error to change the name to that
		// of an existing file.
		err = dir.removeLocked(to)
		if err != nil {
			log.Fatalf("Rename remove failed %v %v\n", to, err)
		}
	}
	err = dir.createLocked(ino, to)
	if err != nil {
		log.Fatalf("Rename create %v failed %v\n", to, err)
		return err
	}

	return nil
}
