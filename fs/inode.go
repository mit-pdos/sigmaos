package fs

import (
	np "ulambda/ninep"
)

type SnapshotF func(Inode) np.Tpath
type RestoreF func(np.Tpath) Inode

// Inode interface for directories

type Inode interface {
	FsObj
	VersionInc()
	SetMtime(int64)
	Mtime() int64
	Size() np.Tlength
	SetParent(Dir)
	Unlink()
	Snapshot(SnapshotF) []byte
}
