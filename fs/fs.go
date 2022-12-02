package fs

import (
	db "sigmaos/debug"
	"sigmaos/fcall"
	np "sigmaos/ninep"
	"sigmaos/npcodec"
	"sigmaos/path"
	"sigmaos/sesscond"
	sp "sigmaos/sigmap"
	"sigmaos/spcodec"
)

type MakeInodeF func(CtxI, sp.Tperm, sp.Tmode, Dir, MakeDirF) (Inode, *fcall.Err)
type MakeDirF func(Inode, MakeInodeF) Inode

type CtxI interface {
	Uname() string
	SessionId() fcall.Tsession
	SessCondTable() *sesscond.SessCondTable
	Snapshot() []byte
}

type Dir interface {
	FsObj
	LookupPath(CtxI, path.Path) ([]FsObj, FsObj, path.Path, *fcall.Err)
	Create(CtxI, string, sp.Tperm, sp.Tmode) (FsObj, *fcall.Err)
	ReadDir(CtxI, int, sp.Tsize, sp.TQversion) ([]*sp.Stat, *fcall.Err)
	WriteDir(CtxI, sp.Toffset, []byte, sp.TQversion) (sp.Tsize, *fcall.Err)
	Remove(CtxI, string) *fcall.Err
	Rename(CtxI, string, string) *fcall.Err
	Renameat(CtxI, string, Dir, string) *fcall.Err
}

type File interface {
	Read(CtxI, sp.Toffset, sp.Tsize, sp.TQversion) ([]byte, *fcall.Err)
	Write(CtxI, sp.Toffset, []byte, sp.TQversion) (sp.Tsize, *fcall.Err)
}

type RPC interface {
	WriteRead(CtxI, []byte) ([]byte, *fcall.Err)
}

type FsObj interface {
	Path() sp.Tpath
	Perm() sp.Tperm
	Parent() Dir
	Open(CtxI, sp.Tmode) (FsObj, *fcall.Err)
	Close(CtxI, sp.Tmode) *fcall.Err // for pipes
	Stat(CtxI) (*sp.Stat, *fcall.Err)
	String() string
}

func Obj2File(o FsObj, fname path.Path) (File, *fcall.Err) {
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

func MarshalDir[Dir *sp.Stat | *np.Stat9P](cnt sp.Tsize, dir []Dir) ([]byte, int, *fcall.Err) {
	var buf []byte

	if len(dir) == 0 {
		return nil, 0, nil
	}
	n := 0
	for _, st := range dir {
		var b []byte
		var e *fcall.Err
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
		cnt -= sp.Tsize(len(b))
		n += 1
	}
	return buf, n, nil
}
