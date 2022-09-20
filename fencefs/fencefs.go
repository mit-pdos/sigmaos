package fencefs

import (
	"sync"

	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/fs"
	"sigmaos/inode"
	np "sigmaos/ninep"
)

//
// An in-memory fs for fences, which is used by fssrv to keep track of
// the most recent fence seen. A fence is named by pathname of its
// epoch file.
//

type Fence struct {
	sync.RWMutex
	fs.Inode
	epoch np.Tepoch
}

func makeFence(i fs.Inode) *Fence {
	e := &Fence{}
	e.Inode = i
	return e
}

func (f *Fence) Write(ctx fs.CtxI, off np.Toffset, b []byte, v np.TQversion) (np.Tsize, *np.Err) {
	return 0, np.MkErr(np.TErrNotSupported, "Write")
}

func (f *Fence) Read(ctx fs.CtxI, off np.Toffset, sz np.Tsize, v np.TQversion) ([]byte, *np.Err) {
	return nil, np.MkErr(np.TErrNotSupported, "Read")
}

func (f *Fence) Snapshot(fn fs.SnapshotF) []byte {
	return makeFenceSnapshot(fn, f)
}

func RestoreFence(fn fs.RestoreF, b []byte) fs.Inode {
	return restoreFence(fn, b)
}

func makeInode(ctx fs.CtxI, p np.Tperm, mode np.Tmode, parent fs.Dir, mk fs.MakeDirF) (fs.Inode, *np.Err) {
	db.DPrintf("FENCEFS", "makeInode %v dir %v\n", p, parent)
	i := inode.MakeInode(ctx, p, parent)
	if p.IsDir() {
		return dir.MakeDir(i, makeInode), nil
	} else if p.IsFile() {
		return makeFence(i), nil
	} else {
		return nil, np.MkErr(np.TErrInval, p)
	}
}

func MakeRoot(ctx fs.CtxI) fs.Dir {
	dir := dir.MkRootDir(ctx, makeInode)
	return dir
}

// XXX check that clnt is allowed to update fence, perhaps using ctx
func allocFence(root fs.Dir, name string) (*Fence, *np.Err) {
	i, err := root.Create(ctx.MkCtx("", 0, nil), name, 0777, np.OWRITE)
	if err == nil {
		f := i.(*Fence)
		f.RLock()
		return f, nil
	}
	if err != nil && err.Code() != np.TErrExists {
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
func CheckFence(root fs.Dir, new np.Tfence) (*Fence, *np.Err) {
	if new.FenceId.Path == 0 {
		return nil, nil
	}
	f, err := allocFence(root, new.FenceId.Path.String())
	if f == nil {
		return nil, err
	}
	if new.Epoch < f.epoch {
		db.DPrintf("FENCES_ERR", "Stale fence %v\n", new)
		f.RUnlock()
		return nil, np.MkErr(np.TErrStale, new)
	}
	if new.Epoch == f.epoch {
		return f, nil
	}

	// Caller has a newer epoch. Upgrade to write lock.
	f.RUnlock()
	f.Lock()

	if new.Epoch > f.epoch {
		db.DPrintf("FENCES", "New epoch %v\n", new)
		f.epoch = new.Epoch
	}

	// Now f.epoch == to new.Epoch. If after down grading this is
	// still true, then we are good to go. Otherwise, f.epoch must
	// have increased, and we return TErrStale.
	f.Unlock()
	f.RLock()
	if new.Epoch == f.epoch {
		return f, nil
	}
	db.DPrintf("FENCES_ERR", "Stale fence %v\n", new)
	return nil, np.MkErr(np.TErrStale, new)
}
