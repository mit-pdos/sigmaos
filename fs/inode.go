package fs

import (
	np "sigmaos/ninep"
)

type SnapshotF func(Inode) np.Tpath
type RestoreF func(np.Tpath) Inode

// Inode interface for directories

type Inode interface {
	FsObj
	SetMtime(int64)
	Mtime() int64
	Size() (np.Tlength, *np.Err)
	SetParent(Dir)
	Unlink()
	Snapshot(SnapshotF) []byte
}
