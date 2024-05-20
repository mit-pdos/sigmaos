// Package fs defines interface for a file system and its objects.
package fs

import (
	"sigmaos/clntcond"
	db "sigmaos/debug"
	np "sigmaos/ninep"
	"sigmaos/npcodec"
	"sigmaos/path"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	"sigmaos/spcodec"
)

type NewFsObjF func(CtxI, sp.Tperm, sp.Tmode, Dir, MkDirF) (FsObj, *serr.Err)
type MkDirF func(Inode, NewFsObjF) FsObj

// Each request takes a Ctx with context for the request
type CtxI interface {
	Principal() *sp.Tprincipal
	Secrets() map[string]*sp.SecretProto
	SessionId() sessp.Tsession
	ClntCondTable() *clntcond.ClntCondTable
	ClntId() sp.TclntId
	FenceFs() Dir
}

// [protsrv] interacts with the backing file system using FsObj.  Backing
// file system include namesrv, ux, s3, and memfs
type FsObj interface {
	Stat(CtxI) (*sp.Stat, *serr.Err)
	Open(CtxI, sp.Tmode) (FsObj, *serr.Err)
	Close(CtxI, sp.Tmode) *serr.Err // for pipes
	Path() sp.Tpath
	Perm() sp.Tperm
	SetParent(Dir)
	Unlink()
	String() string
	Parent() Dir
}

// Two common FsObjs are File and Dir, both which embed an inode
type File interface {
	Inode
	Stat(CtxI) (*sp.Stat, *serr.Err)
	Read(CtxI, sp.Toffset, sp.Tsize, sp.Tfence) ([]byte, *serr.Err)
	Write(CtxI, sp.Toffset, []byte, sp.Tfence) (sp.Tsize, *serr.Err)
}

type Dir interface {
	Inode
	Stat(CtxI) (*sp.Stat, *serr.Err)
	LookupPath(CtxI, path.Path) ([]FsObj, FsObj, path.Path, *serr.Err)
	Create(CtxI, string, sp.Tperm, sp.Tmode, sp.TleaseId, sp.Tfence) (FsObj, *serr.Err)
	ReadDir(CtxI, int, sp.Tsize) ([]*sp.Stat, *serr.Err)
	Remove(CtxI, string, sp.Tfence) *serr.Err
	Rename(CtxI, string, string, sp.Tfence) *serr.Err
	Renameat(CtxI, string, Dir, string, sp.Tfence) *serr.Err
}

type Inode interface {
	Parent() Dir
	Path() sp.Tpath
	Perm() sp.Tperm
	SetMtime(int64)
	Mtime() int64
	SetParent(Dir)
	Unlink()
	NewStat() (*sp.Stat, *serr.Err)
	Open(CtxI, sp.Tmode) (FsObj, *serr.Err)
	Close(CtxI, sp.Tmode) *serr.Err // for pipes
	String() string
}

type RPC interface {
	WriteRead(CtxI, sessp.IoVec) (sessp.IoVec, *serr.Err)
}

func Obj2File(o FsObj, fname path.Path) (File, *serr.Err) {
	switch i := o.(type) {
	case Dir:
		return nil, serr.NewErr(serr.TErrNotFile, fname)
	case File:
		return i, nil
	default:
		db.DFatalf("Obj2File: obj type %T isn't Dir or File\n", o)
	}
	return nil, nil
}

func MarshalDir[Dir *sp.Stat | *np.Stat9P](cnt sp.Tsize, dir []Dir) ([]byte, int, *serr.Err) {
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
			db.DFatalf("MarshalDir unknown type %T\n", st)
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
