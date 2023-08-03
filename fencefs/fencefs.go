package fencefs

import (
	"sync"

	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/fs"
	"sigmaos/inode"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
)

//
// An in-memory fs for fences, which is used by sesssrv to keep track
// of the most recent fence seen. A fence is named by pathname of its
// epoch file.
//

type Fence struct {
	sync.RWMutex
	fs.Inode
	fence sp.Tfence
}

func makeFence(i fs.Inode) *Fence {
	e := &Fence{}
	e.Inode = i
	return e
}

func (f *Fence) Write(ctx fs.CtxI, off sp.Toffset, b []byte, v sp.TQversion, fence sp.Tfence) (sessp.Tsize, *serr.Err) {
	return 0, serr.MkErr(serr.TErrNotSupported, "Write")
}

func (f *Fence) Read(ctx fs.CtxI, off sp.Toffset, sz sessp.Tsize, v sp.TQversion) ([]byte, *serr.Err) {
	return nil, serr.MkErr(serr.TErrNotSupported, "Read")
}

func makeInode(ctx fs.CtxI, p sp.Tperm, mode sp.Tmode, parent fs.Dir, mk fs.MakeDirF) (fs.Inode, *serr.Err) {
	db.DPrintf(db.FENCEFS, "makeInode %v dir %v\n", p, parent)
	i := inode.MakeInode(ctx, p, parent)
	if p.IsDir() {
		return dir.MakeDir(i, makeInode), nil
	} else if p.IsFile() {
		return makeFence(i), nil
	} else {
		return nil, serr.MkErr(serr.TErrInval, p)
	}
}

func MakeRoot(ctx fs.CtxI, parent fs.Dir) fs.Dir {
	dir := dir.MkRootDir(ctx, makeInode, parent)
	return dir
}

// XXX check that clnt is allowed to update fence, perhaps using ctx
func allocFence(root fs.Dir, name string) (*Fence, *serr.Err) {
	i, err := root.Create(ctx.MkCtxNull(), name, 0777, sp.OWRITE, sp.NoLeaseId)
	if err == nil {
		f := i.(*Fence)
		f.RLock()
		return f, nil
	}
	if err != nil && err.Code() != serr.TErrExists {
		db.DFatalf("allocFence create %v err %v\n", name, err)
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
	if root == nil || new.PathName == "" {
		return nil, nil
	}
	f, err := allocFence(root, new.Name())
	if f == nil {
		return nil, err
	}
	if new.LessThan(&f.fence) {
		db.DPrintf(db.FENCEFS_ERR, "Stale fence %v\n", new)
		f.RUnlock()
		return nil, serr.MkErr(serr.TErrStale, new)
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
	return nil, serr.MkErr(serr.TErrStale, new)
}
