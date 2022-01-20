package fs

import (
	np "ulambda/ninep"
	"ulambda/sesscond"
)

type MakeDirF func(FsObj) FsObj

type CtxI interface {
	Uname() string
	SessionId() np.Tsession
	SessCondTable() *sesscond.SessCondTable
}

type Dir interface {
	Lookup(CtxI, []string) ([]FsObj, []string, error)
	Create(CtxI, string, np.Tperm, np.Tmode) (FsObj, error)
	ReadDir(CtxI, np.Toffset, np.Tsize, np.TQversion) ([]*np.Stat, error)
	WriteDir(CtxI, np.Toffset, []byte, np.TQversion) (np.Tsize, error)
	Remove(CtxI, string) error
	Rename(CtxI, string, string) error
	Renameat(CtxI, string, Dir, string) error
}

type File interface {
	Read(CtxI, np.Toffset, np.Tsize, np.TQversion) ([]byte, error)
	Write(CtxI, np.Toffset, []byte, np.TQversion) (np.Tsize, error)
}

type FsObj interface {
	Inum() uint64
	Qid() np.Tqid
	Perm() np.Tperm
	Version() np.TQversion
	VersionInc()
	SetMtime(int64)
	Mtime() int64
	Size() np.Tlength
	Nlink() int
	DecNlink()
	Open(CtxI, np.Tmode) (FsObj, error)
	Close(CtxI, np.Tmode) error // for pipes
	Stat(CtxI) (*np.Stat, error)
	Unlink(CtxI) error
	Parent() Dir
	SetParent(Dir)
}
