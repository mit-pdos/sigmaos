package inode

import (
	"fmt"
	"log"
	"sync"
	"time"
	"unsafe"

	db "ulambda/debug"
	fs "ulambda/fs"
	np "ulambda/ninep"
)

type Tinum uint64
type Tversion uint32

type Inode struct {
	mu      sync.Mutex
	perm    np.Tperm
	version np.TQversion
	mtime   int64
	parent  fs.Dir
	owner   string
	nlink   int
}

func MakeInode(owner string, p np.Tperm, parent fs.Dir) *Inode {
	i := Inode{}
	i.perm = p
	i.mtime = time.Now().Unix()
	i.parent = parent
	i.owner = owner
	i.version = np.TQversion(1)
	i.nlink = 1
	return &i
}

func (inode *Inode) String() string {
	str := fmt.Sprintf("Inode %p %v", inode, inode.perm)
	return str
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
	inode.mu.Lock()
	defer inode.mu.Unlock()
	return inode.qidL()
}

func (inode *Inode) Perm() np.Tperm {
	return inode.perm
}

func (inode *Inode) Parent() fs.Dir {
	inode.mu.Lock()
	defer inode.mu.Unlock()
	return inode.parent
}

func (inode *Inode) Version() np.TQversion {
	inode.mu.Lock()
	defer inode.mu.Unlock()

	return inode.version
}

func (inode *Inode) VersionInc() {
	inode.mu.Lock()
	defer inode.mu.Unlock()

	inode.version += 1
}

func (inode *Inode) Nlink() int {
	inode.mu.Lock()
	defer inode.mu.Unlock()

	return inode.nlink
}

func (inode *Inode) DecNlink() {
	inode.mu.Lock()
	defer inode.mu.Unlock()

	inode.nlink--
}

func (inode *Inode) SetParent(p fs.Dir) {
	inode.mu.Lock()
	defer inode.mu.Unlock()
	inode.parent = p
}

func (inode *Inode) Mtime() int64 {
	inode.mu.Lock()
	defer inode.mu.Unlock()
	return inode.mtime
}

func (inode *Inode) SetMtime(m int64) {
	inode.mu.Lock()
	defer inode.mu.Unlock()
	inode.mtime = m
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

func (i *Inode) Unlink(ctx fs.CtxI) error {
	i.nlink -= 1
	if i.nlink < 0 {
		log.Printf("%v: nlink < 0\n", db.GetName())
	}
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
	inode.mu.Lock()
	defer inode.mu.Unlock()

	stat := &np.Stat{}
	stat.Type = 0 // XXX
	stat.Qid = inode.qidL()
	stat.Mode = inode.Mode()
	stat.Mtime = uint32(inode.mtime)
	stat.Atime = 0
	stat.Name = ""
	stat.Uid = inode.owner
	stat.Gid = inode.owner
	stat.Muid = ""
	return stat, nil
}
