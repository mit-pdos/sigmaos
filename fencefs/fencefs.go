// Package fencefs provides an in-memory fs for fences, which is used
// by sigmasrv to keep track of the most recent fence seen. A fence is
// named by pathname of its epoch file.
package fencefs

import (
	"sync"

	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/memfs/dir"
	"sigmaos/memfs/inode"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

type Fence struct {
	sync.RWMutex
	fs.Inode
	fence sp.Tfence
}

func newFence(i fs.Inode) *Fence {
	f := &Fence{}
	f.Inode = i
	return f
}

func (f *Fence) Stat(ctx fs.CtxI) (*sp.Stat, *serr.Err) {
	return nil, serr.NewErr(serr.TErrNotSupported, "Stat")
}

func (f *Fence) Write(ctx fs.CtxI, off sp.Toffset, b []byte, fence sp.Tfence) (sp.Tsize, *serr.Err) {
	return 0, serr.NewErr(serr.TErrNotSupported, "Write")
}

func (f *Fence) Read(ctx fs.CtxI, off sp.Toffset, sz sp.Tsize, fence sp.Tfence) ([]byte, *serr.Err) {
	return nil, serr.NewErr(serr.TErrNotSupported, "Read")
}

func newInode(ctx fs.CtxI, p sp.Tperm, lid sp.TleaseId, mode sp.Tmode, parent fs.Dir, new fs.MkDirF) (fs.FsObj, *serr.Err) {
	db.DPrintf(db.FENCEFS, "newInode %v dir %v\n", p, parent)
	i := inode.NewInode(ctx, p, lid, parent)
	if p.IsDir() {
		return dir.MkDir(i, newInode), nil
	} else if p.IsFile() {
		return newFence(i), nil
	} else {
		return nil, serr.NewErr(serr.TErrInval, p)
	}
}

func NewRoot(ctx fs.CtxI, parent fs.Dir) fs.Dir {
	dir := dir.NewRootDir(ctx, newInode, parent)
	return dir
}

// XXX check that clnt is allowed to update fence, perhaps using ctx
func allocFence(root fs.Dir, name string) (*Fence, *serr.Err) {
	i, err := root.Create(ctx.NewCtxNull(), name, 0777, sp.OWRITE, sp.NoLeaseId, sp.NoFence(), nil)
	if err == nil {
		f := i.(*Fence)
		f.RLock()
		return f, nil
	}
	if err != nil && err.Code() != serr.TErrExists {
		db.DPrintf(db.ERROR, "allocFence create %v err %v\n", name, err)
		return nil, err
	}
	f := i.(*Fence)
	f.RLock()
	return f, err
}

// If new is NoFence, return. If no fence exists for new's fence id,
// store it as the most recent fence.  If the fence exists but new is
// newer, update the fence.  If new is stale, return error.  If fence
// id exists, return the locked fence in read mode so that no one can
// update the fence until this fenced operation has completed. Read
// mode so that we can run operations in the same epoch in parallel.
func CheckFence(root fs.Dir, new sp.Tfence) (*Fence, *serr.Err) {
	if root == nil || !new.HasFence() {
		return nil, nil
	}
	f, err := allocFence(root, new.Name())
	if f == nil {
		return nil, err
	}
	db.DPrintf(db.FENCEFS, "CheckFence f %v new %v\n", f.fence, new)
	if new.LessThan(&f.fence) {
		db.DPrintf(db.FENCEFS_ERR, "Stale fence %v\n", new)
		f.RUnlock()
		return nil, serr.NewErr(serr.TErrStale, new)
	}
	if new.Eq(&f.fence) {
		return f, nil
	}

	// Caller has a newer epoch. Upgrade to write lock.
	f.RUnlock()
	f.Lock()

	db.DPrintf(db.FENCEFS, "New epoch %v\n", new)
	f.fence.Upgrade(&new)

	// Now f == new. If after down grading this is still true, then we
	// are good to go. Otherwise, f must have increased, and we return
	// TErrStale.
	f.Unlock()
	f.RLock()
	if f.fence.Eq(&new) {
		return f, nil
	}
	db.DPrintf(db.FENCEFS_ERR, "Stale fence %v\n", new)
	return nil, serr.NewErr(serr.TErrStale, new)
}
