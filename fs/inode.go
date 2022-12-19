package fs

import (
	sp "sigmaos/sigmap"
    "sigmaos/fcall"
)

type SnapshotF func(Inode) sp.Tpath
type RestoreF func(sp.Tpath) Inode

// Inode interface for directories

type Inode interface {
	FsObj
	SetMtime(int64)
	Mtime() int64
	Size() (sp.Tlength, *fcall.Err)
	SetParent(Dir)
	Unlink()
	Snapshot(SnapshotF) []byte
}
