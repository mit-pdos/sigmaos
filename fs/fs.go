package fs

import (
	db "sigmaos/debug"
	"sigmaos/sessp"
    "sigmaos/serr"
	np "sigmaos/ninep"
	"sigmaos/npcodec"
	"sigmaos/path"
	"sigmaos/sesscond"
	sp "sigmaos/sigmap"
	"sigmaos/spcodec"
)

type MakeInodeF func(CtxI, sp.Tperm, sp.Tmode, Dir, MakeDirF) (Inode, *serr.Err)
type MakeDirF func(Inode, MakeInodeF) Inode

type CtxI interface {
	Uname() string
	SessionId() sessp.Tsession
	SessCondTable() *sesscond.SessCondTable
	Snapshot() []byte
}

type Dir interface {
	FsObj
	LookupPath(CtxI, path.Path) ([]FsObj, FsObj, path.Path, *serr.Err)
	Create(CtxI, string, sp.Tperm, sp.Tmode) (FsObj, *serr.Err)
	ReadDir(CtxI, int, sessp.Tsize, sp.TQversion) ([]*sp.Stat, *serr.Err)
	WriteDir(CtxI, sp.Toffset, []byte, sp.TQversion) (sessp.Tsize, *serr.Err)
	Remove(CtxI, string) *serr.Err
	Rename(CtxI, string, string) *serr.Err
	Renameat(CtxI, string, Dir, string) *serr.Err
}

type File interface {
	Read(CtxI, sp.Toffset, sessp.Tsize, sp.TQversion) ([]byte, *serr.Err)
	Write(CtxI, sp.Toffset, []byte, sp.TQversion) (sessp.Tsize, *serr.Err)
}

type RPC interface {
	WriteRead(CtxI, []byte) ([]byte, *serr.Err)
}

type FsObj interface {
	Path() sessp.Tpath
	Perm() sp.Tperm
	Parent() Dir
	Open(CtxI, sp.Tmode) (FsObj, *serr.Err)
	Close(CtxI, sp.Tmode) *serr.Err // for pipes
	Stat(CtxI) (*sp.Stat, *serr.Err)
	String() string
}

func Obj2File(o FsObj, fname path.Path) (File, *serr.Err) {
	switch i := o.(type) {
	case Dir:
		return nil, serr.MkErr(serr.TErrNotFile, fname)
	case File:
		return i, nil
	default:
		db.DFatalf("Obj2File: obj type %T isn't Dir or File\n", o)
	}
	return nil, nil
}

func MarshalDir[Dir *sp.Stat | *np.Stat9P](cnt sessp.Tsize, dir []Dir) ([]byte, int, *serr.Err) {
	var buf []byte

	if len(dir) == 0 {
		return nil, 0, nil
	}
	n := 0
	for _, st := range dir {
		var b []byte
		var e *serr.Err
		switch any(st).(type) {
		case *np.Stat9P:
			b, e = npcodec.MarshalDirEnt(any(st).(*np.Stat9P), uint64(cnt))
		case *sp.Stat:
			b, e = spcodec.MarshalDirEnt(any(st).(*sp.Stat), uint64(cnt))
		default:
			db.DFatalf("MARSHAL", "MarshalDir unknown type %T\n", st)
		}
		if e != nil {
			return nil, 0, e
		}
		if b == nil {
			break
		}

		buf = append(buf, b...)
		cnt -= sessp.Tsize(len(b))
		n += 1
	}
	return buf, n, nil
}
