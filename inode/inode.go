package inode

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"ulambda/fs"
	np "ulambda/ninep"
)

type Tversion uint32

type Inode struct {
	mu      sync.Mutex
	inum    uint64
	perm    np.Tperm
	version np.TQversion
	mtime   int64
	parent  fs.Dir
	owner   string
}

var nextInum uint64 = 0

func MakeInode(ctx fs.CtxI, p np.Tperm, parent fs.Dir) *Inode {
	i := &Inode{}
	// This may be overly defensive, but setting an inode's inum as it's address
	// seems risky, as the go GC moves objects and may accidentally make two
	// objects have the same address.
	i.inum = atomic.AddUint64(&nextInum, 1)
	i.perm = p
	i.mtime = time.Now().Unix()
	i.parent = parent
	if ctx == nil {
		i.owner = ""
	} else {
		i.owner = ctx.Uname()
	}
	i.version = np.TQversion(1)
	return i
}

func (inode *Inode) String() string {
	str := fmt.Sprintf("Inode %p %v", inode, inode.perm)
	return str
}

func (inode *Inode) Inum() uint64 {
	return inode.inum
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

func (i *Inode) Open(ctx fs.CtxI, mode np.Tmode) (fs.FsObj, *np.Err) {
	return nil, nil
}

func (i *Inode) Close(ctx fs.CtxI, mode np.Tmode) *np.Err {
	return nil
}

func (i *Inode) Unlink() {
}

func (inode *Inode) Mode() np.Tperm {
	perm := np.Tperm(0777)
	if inode.perm.IsDir() {
		perm |= np.DMDIR
	}
	return perm
}

func (inode *Inode) Stat(ctx fs.CtxI) (*np.Stat, *np.Err) {
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

func (inode *Inode) Snapshot(fn fs.SnapshotF) []byte {
	return makeSnapshot(inode)
}

func RestoreInode(f fs.RestoreF, b []byte) fs.Inode {
	return restoreInode(f, b)
}
