package fs

import (
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

// Inode interface for directories

type Inode interface {
	FsObj
	SetMtime(int64)
	Mtime() int64
	Size() (sp.Tlength, *serr.Err)
	SetParent(Dir)
	Unlink()
}
