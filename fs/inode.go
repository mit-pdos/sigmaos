package fs

import (
	np "sigmaos/sigmap"
    "sigmaos/fcall"
)

type SnapshotF func(Inode) np.Tpath
type RestoreF func(np.Tpath) Inode

// Inode interface for directories

type Inode interface {
	FsObj
	SetMtime(int64)
	Mtime() int64
	Size() (np.Tlength, *fcall.Err)
	SetParent(Dir)
	Unlink()
	Snapshot(SnapshotF) []byte
}
