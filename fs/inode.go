package fs

import (
	sp "sigmaos/sigmap"
    "sigmaos/fcall"
)

type SnapshotF func(Inode) fcall.Tpath
type RestoreF func(fcall.Tpath) Inode

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
