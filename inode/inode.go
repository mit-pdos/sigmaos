package inode

import (
	"fmt"
	"sync"
	"time"
	"unsafe"

	fs "ulambda/fs"
	np "ulambda/ninep"
)

type Tinum uint64
type Tversion uint32

type Inode struct {
	mu      sync.Mutex
	perm    np.Tperm
	version np.TQversion
	Mtime   int64
	parent  fs.Dir
	owner   string
}

func MakeInode(owner string, p np.Tperm, parent fs.Dir) *Inode {
	i := Inode{}
	i.perm = p
	i.Mtime = time.Now().Unix()
	i.parent = parent
	i.owner = owner
	i.version = np.TQversion(1)
	return &i
}

func (inode *Inode) String() string {
	str := fmt.Sprintf("Inode %p %v", inode, inode.perm)
	return str
}

func (inode *Inode) Lock() {
	inode.mu.Lock()
}

func (inode *Inode) Unlock() {
	inode.mu.Unlock()
}

func (inode *Inode) LockAddr() *sync.Mutex {
	return &inode.mu
}

func (inode *Inode) Inum() uint64 {
	id := uintptr(unsafe.Pointer(inode))
	return uint64(id)
}

func (inode *Inode) qidL() np.Tqid {
	return np.MakeQid(
		np.Qtype(inode.perm>>np.QTYPESHIFT),
		np.TQversion(inode.version),
		np.Tpath(inode.Inum()))
}

func (inode *Inode) Qid() np.Tqid {
	inode.Lock()
	defer inode.Unlock()
	return inode.qidL()
}

func (inode *Inode) Perm() np.Tperm {
	inode.mu.Lock()
	defer inode.mu.Unlock()
	return inode.perm
}

func (inode *Inode) Parent() fs.Dir {
	return inode.parent
}

func (inode *Inode) Version() np.TQversion {
	return inode.version
}

func (inode *Inode) VersionInc() {
	inode.version += 1
}

func (inode *Inode) SetParent(p fs.Dir) {
	inode.parent = p
}

func (inode *Inode) SetMtime() {
	inode.Mtime = time.Now().Unix()
}

func (i *Inode) Size() np.Tlength {
	return 0
}

func (i *Inode) Open(ctx fs.CtxI, mode np.Tmode) (fs.FsObj, error) {
	return nil, nil
}

func (i *Inode) Close(ctx fs.CtxI, mode np.Tmode) error {
	return nil
}

func (inode *Inode) Mode() np.Tperm {
	perm := np.Tperm(0777)
	if inode.perm.IsDir() {
		perm |= np.DMDIR
	}
	return perm
}

func (inode *Inode) Stat(ctx fs.CtxI) (*np.Stat, error) {
	stat := &np.Stat{}
	stat.Type = 0 // XXX
	stat.Qid = inode.qidL()
	stat.Mode = inode.Mode()
	stat.Mtime = uint32(inode.Mtime)
	stat.Atime = 0
	stat.Name = ""
	stat.Uid = inode.owner
	stat.Gid = inode.owner
	stat.Muid = ""
	return stat, nil
}
