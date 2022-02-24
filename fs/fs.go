package fs

import (
	"log"

	np "ulambda/ninep"
	"ulambda/sesscond"
)

type MakeDirF func(FsObj) FsObj
type SnapshotF func(FsObj) uintptr
type RestoreF func(uintptr) FsObj

type CtxI interface {
	Uname() string
	SessionId() np.Tsession
	SessCondTable() *sesscond.SessCondTable
}

type Dir interface {
	Lookup(CtxI, []string) ([]FsObj, []string, *np.Err)
	Create(CtxI, string, np.Tperm, np.Tmode) (FsObj, *np.Err)
	ReadDir(CtxI, int, np.Tsize, np.TQversion) ([]*np.Stat, *np.Err)
	WriteDir(CtxI, np.Toffset, []byte, np.TQversion) (np.Tsize, *np.Err)
	Remove(CtxI, string) *np.Err
	Rename(CtxI, string, string) *np.Err
	Renameat(CtxI, string, Dir, string) *np.Err
}

type File interface {
	Read(CtxI, np.Toffset, np.Tsize, np.TQversion) ([]byte, *np.Err)
	Write(CtxI, np.Toffset, []byte, np.TQversion) (np.Tsize, *np.Err)
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
	Open(CtxI, np.Tmode) (FsObj, *np.Err)
	Close(CtxI, np.Tmode) *np.Err // for pipes
	Stat(CtxI) (*np.Stat, *np.Err)
	Unlink(CtxI) *np.Err
	Parent() Dir
	SetParent(Dir)
}

func Obj2File(o FsObj, fname []string) (File, *np.Err) {
	switch i := o.(type) {
	case Dir:
		return nil, np.MkErr(np.TErrNotFile, fname)
	case File:
		return i, nil
	default:
		log.Fatalf("FATAL Obj2File: obj type %T isn't Dir or File\n", o)
	}
	return nil, nil
}
