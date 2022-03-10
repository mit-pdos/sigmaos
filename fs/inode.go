package fs

import (
	np "ulambda/ninep"
)

type SnapshotF func(Inode) uint64
type RestoreF func(uint64) Inode

type MakeDirF func(Inode) Inode

// Inode interface for directories

type Inode interface {
	FsObj
	VersionInc()
	SetMtime(int64)
	Mtime() int64
	Size() np.Tlength
	Nlink() int
	DecNlink()
	Unlink(CtxI) *np.Err
	SetParent(Dir)
}
