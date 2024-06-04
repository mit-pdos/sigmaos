// Package inode implements the Inode inteface for an in-memory inode.
package inode

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"sigmaos/fs"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type Inode struct {
	mu     sync.Mutex
	inum   sp.Tpath
	perm   sp.Tperm
	lid    sp.TleaseId
	mtime  int64
	parent fs.Dir
	owner  *sp.Tprincipal
}

var NextInum atomic.Uint64

func NewInode(ctx fs.CtxI, p sp.Tperm, lid sp.TleaseId, parent fs.Dir) *Inode {
	i := &Inode{
		inum:   sp.Tpath(NextInum.Add(1)),
		perm:   p,
		mtime:  time.Now().Unix(),
		parent: parent,
		lid:    lid,
	}
	if ctx == nil {
		i.owner = sp.NoPrincipal()
	} else {
		i.owner = ctx.Principal()
	}
	return i
}

func (inode *Inode) String() string {
	inode.mu.Lock()
	defer inode.mu.Unlock()
	str := fmt.Sprintf("{ino %p inum %v %v}", inode, inode.inum, inode.perm)
	return str
}

func (inode *Inode) NewStat() (*sp.Stat, *serr.Err) {
	inode.mu.Lock()
	defer inode.mu.Unlock()

	return sp.NewStat(sp.NewQidPerm(inode.perm, 0, inode.inum),
		inode.Mode(), uint32(inode.mtime), "", inode.owner.GetID().String()), nil
}

func (inode *Inode) Path() sp.Tpath {
	inode.mu.Lock()
	defer inode.mu.Unlock()
	return inode.inum
}

func (inode *Inode) Perm() sp.Tperm {
	inode.mu.Lock()
	defer inode.mu.Unlock()
	return inode.perm
}

func (inode *Inode) IsLeased() bool {
	inode.mu.Lock()
	defer inode.mu.Unlock()
	return inode.lid.IsLeased()
}

func (inode *Inode) Parent() fs.Dir {
	inode.mu.Lock()
	defer inode.mu.Unlock()
	return inode.parent
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

func (i *Inode) Open(ctx fs.CtxI, mode sp.Tmode) (fs.FsObj, *serr.Err) {
	return nil, nil
}

func (i *Inode) Close(ctx fs.CtxI, mode sp.Tmode) *serr.Err {
	return nil
}

func (i *Inode) Unlink() {
}

func (inode *Inode) Mode() sp.Tperm {
	perm := sp.Tperm(0777)
	if inode.perm.IsDir() {
		perm |= sp.DMDIR
	}
	return perm
}
