package fs

import (
	"sync"

	np "ulambda/ninep"
)

type MakeDirF func(FsObj) FsObj

type CtxI interface {
	Uname() string
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
	SetMtime()
	Size() np.Tlength
	Open(CtxI, np.Tmode) (FsObj, error)
	Close(CtxI, np.Tmode) error // for pipes
	Stat(CtxI) (*np.Stat, error)
	Parent() Dir
	SetParent(Dir)
	Lock()
	Unlock()
	LockAddr() *sync.Mutex
}
