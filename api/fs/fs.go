// Package fs defines interface for a file system and its objects.
package fs

import (
	db "sigmaos/debug"
	np "sigmaos/ninep"
	"sigmaos/path"
	"sigmaos/proxy/ninep/npcodec"
	"sigmaos/serr"
	spcodec "sigmaos/session/codec"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv/clntcond"
)

type NewFsObjF func(CtxI, sp.Tperm, sp.TleaseId, sp.Tmode, MkDirF) (FsObj, *serr.Err)
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
	Stat(CtxI) (*sp.Tstat, *serr.Err)
	Open(CtxI, sp.Tmode) (FsObj, *serr.Err)
	Close(CtxI, sp.Tmode) *serr.Err // for pipes
	Path() sp.Tpath
	Perm() sp.Tperm
	Unlink()
	String() string
	IsLeased() bool
}

// Two common FsObjs are File and Dir, both which embed an inode
type File interface {
	Inode
	Stat(CtxI) (*sp.Tstat, *serr.Err)
	Read(CtxI, sp.Toffset, sp.Tsize, sp.Tfence) ([]byte, *serr.Err)
	Write(CtxI, sp.Toffset, []byte, sp.Tfence) (sp.Tsize, *serr.Err)
}

type Tdel int

const (
	DEL_EXIST Tdel = iota + 1
	DEL_EPHEMERAL
)

type Dir interface {
	Inode
	Stat(CtxI) (*sp.Tstat, *serr.Err)
	LookupPath(CtxI, path.Tpathname) ([]FsObj, FsObj, path.Tpathname, *serr.Err)
	Create(CtxI, string, sp.Tperm, sp.Tmode, sp.TleaseId, sp.Tfence, FsObj) (FsObj, *serr.Err)
	ReadDir(CtxI, int, sp.Tsize) ([]*sp.Tstat, *serr.Err)
	Remove(CtxI, string, sp.Tfence, Tdel) *serr.Err
	Rename(CtxI, string, string, sp.Tfence) *serr.Err
	Renameat(CtxI, string, Dir, string, sp.Tfence) *serr.Err
}

type Inode interface {
	Path() sp.Tpath
	Perm() sp.Tperm
	IsLeased() bool
	SetMtime(int64)
	Mtime() int64
	Unlink()
	NewStat() (*sp.Tstat, *serr.Err)
	Open(CtxI, sp.Tmode) (FsObj, *serr.Err)
	Close(CtxI, sp.Tmode) *serr.Err // for pipes
	String() string
}

type RPC interface {
	WriteRead(CtxI, sessp.IoVec) (sessp.IoVec, *serr.Err)
}

func Obj2File(o FsObj, fname path.Tpathname) (File, *serr.Err) {
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

func MarshalDir[Dir *sp.Tstat | *np.Stat9P](cnt sp.Tsize, dir []Dir) ([]byte, int, *serr.Err) {
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
		case *sp.Tstat:
			b, e = spcodec.MarshalDirEnt(any(st).(*sp.Tstat), uint64(cnt))
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
