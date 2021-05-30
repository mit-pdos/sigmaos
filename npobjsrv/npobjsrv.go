package npobjsrv

import (
	np "ulambda/ninep"
)

type NpObjSrv interface {
	// Maybe pass uname to RootAttach()
	RootAttach(string) (NpObj, CtxI)
	Done()
	WatchTable() *WatchTable
	ConnTable() *ConnTable
	Stats() *Stats
}

type CtxI interface {
	Uname() string
}

type NpObjDir interface {
	Lookup(CtxI, []string) ([]NpObj, []string, error)
	Create(CtxI, string, np.Tperm, np.Tmode) (NpObj, error)
	ReadDir(CtxI, np.Toffset, np.Tsize, np.TQversion) ([]*np.Stat, error)
	WriteDir(CtxI, np.Toffset, []byte, np.TQversion) (np.Tsize, error)
	Renameat(CtxI, string, NpObjDir, string) error
}

type NpObjFile interface {
	Read(CtxI, np.Toffset, np.Tsize, np.TQversion) ([]byte, error)
	Write(CtxI, np.Toffset, []byte, np.TQversion) (np.Tsize, error)
}

type NpObj interface {
	Qid() np.Tqid
	Perm() np.Tperm
	Version() np.TQversion
	Size() np.Tlength
	Open(CtxI, np.Tmode) error
	Close(CtxI, np.Tmode) error
	Remove(CtxI, string) error
	Stat(CtxI) (*np.Stat, error)
	Rename(CtxI, string, string) error
}
