package fencefs

import (
	"sync"

	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/fcall"
	"sigmaos/fs"
	"sigmaos/inode"
	sp "sigmaos/sigmap"
)

//
// An in-memory fs for fences, which is used by fssrv to keep track of
// the most recent fence seen. A fence is named by pathname of its
// epoch file.
//

type Fence struct {
	sync.RWMutex
	fs.Inode
	epoch sp.Tepoch
}

func makeFence(i fs.Inode) *Fence {
	e := &Fence{}
	e.Inode = i
	return e
}

func (f *Fence) Write(ctx fs.CtxI, off sp.Toffset, b []byte, v sp.TQversion) (sp.Tsize, *fcall.Err) {
	return 0, fcall.MkErr(fcall.TErrNotSupported, "Write")
}

func (f *Fence) Read(ctx fs.CtxI, off sp.Toffset, sz sp.Tsize, v sp.TQversion) ([]byte, *fcall.Err) {
	return nil, fcall.MkErr(fcall.TErrNotSupported, "Read")
}

func (f *Fence) Snapshot(fn fs.SnapshotF) []byte {
	return makeFenceSnapshot(fn, f)
}

func RestoreFence(fn fs.RestoreF, b []byte) fs.Inode {
	return restoreFence(fn, b)
}

func makeInode(ctx fs.CtxI, p sp.Tperm, mode sp.Tmode, parent fs.Dir, mk fs.MakeDirF) (fs.Inode, *fcall.Err) {
	db.DPrintf(db.FENCEFS, "makeInode %v dir %v\n", p, parent)
	i := inode.MakeInode(ctx, p, parent)
	if p.IsDir() {
		return dir.MakeDir(i, makeInode), nil
	} else if p.IsFile() {
		return makeFence(i), nil
	} else {
		return nil, fcall.MkErr(fcall.TErrInval, p)
	}
}

func MakeRoot(ctx fs.CtxI) fs.Dir {
	dir := dir.MkRootDir(ctx, makeInode)
	return dir
}

// XXX check that clnt is allowed to update fence, perhaps using ctx
func allocFence(root fs.Dir, name string) (*Fence, *fcall.Err) {
	i, err := root.Create(ctx.MkCtx("", 0, nil), name, 0777, sp.OWRITE)
	if err == nil {
		f := i.(*Fence)
		f.RLock()
		return f, nil
	}
	if err != nil && err.Code() != fcall.TErrExists {
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
func CheckFence(root fs.Dir, new sp.Tfence) (*Fence, *fcall.Err) {
	if new.Fenceid.Path == 0 {
		return nil, nil
	}
	f, err := allocFence(root, new.Fenceid.Tpath().String())
	if f == nil {
		return nil, err
	}
	e := new.Tepoch()
	if e < f.epoch {
		db.DPrintf(db.FENCEFS_ERR, "Stale fence %v\n", new)
		f.RUnlock()
		return nil, fcall.MkErr(fcall.TErrStale, new)
	}
	if e == f.epoch {
		return f, nil
	}

	// Caller has a newer epoch. Upgrade to write lock.
	f.RUnlock()
	f.Lock()

	if e > f.epoch {
		db.DPrintf(db.FENCEFS, "New epoch %v\n", new)
		f.epoch = e
	}

	// Now f.epoch == to new.Epoch. If after down grading this is
	// still true, then we are good to go. Otherwise, f.epoch must
	// have increased, and we return TErrStale.
	f.Unlock()
	f.RLock()
	if e == f.epoch {
		return f, nil
	}
	db.DPrintf(db.FENCEFS_ERR, "Stale fence %v\n", new)
	return nil, fcall.MkErr(fcall.TErrStale, new)
}
