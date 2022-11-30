package fs

import (
	db "sigmaos/debug"
	np "sigmaos/sigmap"
    "sigmaos/fcall"
	"sigmaos/sesscond"
)

type MakeInodeF func(CtxI, np.Tperm, np.Tmode, Dir, MakeDirF) (Inode, *fcall.Err)
type MakeDirF func(Inode, MakeInodeF) Inode

type CtxI interface {
	Uname() string
	SessionId() fcall.Tsession
	SessCondTable() *sesscond.SessCondTable
	Snapshot() []byte
}

type Dir interface {
	FsObj
	LookupPath(CtxI, np.Path) ([]FsObj, FsObj, np.Path, *fcall.Err)
	Create(CtxI, string, np.Tperm, np.Tmode) (FsObj, *fcall.Err)
	ReadDir(CtxI, int, np.Tsize, np.TQversion) ([]*np.Stat, *fcall.Err)
	WriteDir(CtxI, np.Toffset, []byte, np.TQversion) (np.Tsize, *fcall.Err)
	Remove(CtxI, string) *fcall.Err
	Rename(CtxI, string, string) *fcall.Err
	Renameat(CtxI, string, Dir, string) *fcall.Err
}

type File interface {
	Read(CtxI, np.Toffset, np.Tsize, np.TQversion) ([]byte, *fcall.Err)
	Write(CtxI, np.Toffset, []byte, np.TQversion) (np.Tsize, *fcall.Err)
}

type RPC interface {
	WriteRead(CtxI, []byte) ([]byte, *fcall.Err)
}

type FsObj interface {
	Path() np.Tpath
	Perm() np.Tperm
	Parent() Dir
	Open(CtxI, np.Tmode) (FsObj, *fcall.Err)
	Close(CtxI, np.Tmode) *fcall.Err // for pipes
	Stat(CtxI) (*np.Stat, *fcall.Err)
	String() string
}

func Obj2File(o FsObj, fname np.Path) (File, *fcall.Err) {
	switch i := o.(type) {
	case Dir:
		return nil, fcall.MkErr(fcall.TErrNotFile, fname)
	case File:
		return i, nil
	default:
		db.DFatalf("Obj2File: obj type %T isn't Dir or File\n", o)
	}
	return nil, nil
}
