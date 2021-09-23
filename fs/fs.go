package fs

import (
	np "ulambda/ninep"
)

type CtxI interface {
	Uname() string
}

type Dir interface {
	Lookup(CtxI, []string) ([]FsObj, []string, error)
	Create(CtxI, string, np.Tperm, np.Tmode) (FsObj, error)
	ReadDir(CtxI, np.Toffset, np.Tsize, np.TQversion) ([]*np.Stat, error)
	WriteDir(CtxI, np.Toffset, []byte, np.TQversion) (np.Tsize, error)
	Renameat(CtxI, string, Dir, string) error
}

type File interface {
	Read(CtxI, np.Toffset, np.Tsize, np.TQversion) ([]byte, error)
	Write(CtxI, np.Toffset, []byte, np.TQversion) (np.Tsize, error)
}

type FsObj interface {
	Qid() np.Tqid
	Perm() np.Tperm
	Version() np.TQversion
	Size() np.Tlength
	Open(CtxI, np.Tmode) (FsObj, error)
	Close(CtxI, np.Tmode) error // for pipes
	Remove(CtxI, string) error
	Stat(CtxI) (*np.Stat, error)
	Rename(CtxI, string, string) error
}
