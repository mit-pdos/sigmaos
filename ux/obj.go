//go:build linux
// +build linux

package fsux

import (
	"fmt"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/path"
	"sigmaos/serr"
	sp "sigmaos/sigmap"
)

func statxTimestampToTime(sts unix.StatxTimestamp) time.Time {
	return time.Unix(sts.Sec, int64(sts.Nsec))
}

func newQid(mode sp.Tperm, v sp.TQversion, path sp.Tpath) *sp.Tqid {
	return sp.NewQid(sp.Qtype(mode>>sp.QTYPESHIFT), v, path)
}

func umode2Perm(umode uint16) sp.Tperm {
	perm := sp.Tperm(umode & 0777)
	switch umode & syscall.S_IFMT {
	case syscall.S_IFREG:
		// file
	case syscall.S_IFDIR:
		perm |= sp.DMDIR
	case syscall.S_IFIFO:
		perm |= sp.DMNAMEDPIPE
	case syscall.S_IFLNK:
		perm |= sp.DMSYMLINK
	}
	db.DPrintf(db.UX, "mode 0%o type 0%o perm %v", umode, umode&syscall.S_IFMT, perm)
	return perm
}

func ustat(path path.Tpathname) (*sp.Stat, *serr.Err) {
	var statx unix.Statx_t
	db.DPrintf(db.UX, "ustat %v\n", path)
	if error := unix.Statx(unix.AT_FDCWD, path.String(), unix.AT_SYMLINK_NOFOLLOW, unix.STATX_ALL, &statx); error != nil {
		db.DPrintf(db.UX, "ustat %v err %v\n", path, error)
		return nil, serr.UxErrnoToErr(error, path.Base())
	}
	t := statxTimestampToTime(statx.Mtime)
	st := sp.NewStat(sp.NewQidPerm(umode2Perm(statx.Mode), 0, sp.Tpath(statx.Ino)),
		umode2Perm(statx.Mode), uint32(t.Unix()), path.Base(), "")
	st.SetLength(sp.Tlength(statx.Size))
	return st, nil
}

type Obj struct {
	pathName path.Tpathname
	path     sp.Tpath
	perm     sp.Tperm // XXX kill, but requires changing Perm() API
}

func (o *Obj) String() string {
	return fmt.Sprintf("pn %v p %v %v", o.pathName, o.path, o.perm)
}

func newObj(path path.Tpathname) (*Obj, *serr.Err) {
	if st, err := ustat(path); err != nil {
		return &Obj{path, 0, sp.DMSYMLINK}, err
	} else {
		return &Obj{path, st.Tqid().Tpath(), st.Tmode()}, nil
	}
}

func (o *Obj) Perm() sp.Tperm {
	return o.perm
	// st, err := ustat(o.pathName)
	// if err != nil {
	// 	db.DPrintf(db.UX, "Perm %v err %v\n", o.pathName, err)
	// 	return nil, err
	// }
	//db.DPrintf(db.UX, "Perm %v st %v\n", o.pathName, st)
	//return st.Mode, nil
}

func (o *Obj) Path() sp.Tpath {
	return o.path
}

func (o *Obj) PathName() string {
	p := o.pathName.String()
	if len(o.pathName) == 0 {
		p = "."
	}
	return p
}

func (o *Obj) NewStat() (*sp.Stat, *serr.Err) {
	db.DPrintf(db.UX, "%v: NewStat\n", o)
	st, err := ustat(o.pathName)
	if err != nil {
		return nil, err
	}
	return st, nil
}

func (o *Obj) Stat(ctx fs.CtxI) (*sp.Stat, *serr.Err) {
	db.DPrintf(db.UX, "%v: Stat %v\n", ctx, o)
	st, err := o.NewStat()
	if err != nil {
		return nil, err
	}
	return st, nil
}

func uxFlags(m sp.Tmode) int {
	f := syscall.O_NOFOLLOW
	switch m & 3 {
	case sp.OREAD:
		f |= syscall.O_RDONLY
	case sp.ORDWR:
		f |= syscall.O_RDWR
	case sp.OWRITE:
		f |= syscall.O_WRONLY
	case sp.OEXEC:
		f |= syscall.O_RDONLY
	}
	if m&sp.OTRUNC == sp.OTRUNC {
		f |= syscall.O_TRUNC
	}
	if m&sp.OAPPEND == sp.OAPPEND {
		f |= syscall.O_APPEND
		f |= syscall.O_WRONLY
	}
	return f
}

//
// Inode interface
//

func (o *Obj) Mtime() int64 {
	return 0
}

func (o *Obj) SetMtime(m int64) {
}

func (o *Obj) Parent() fs.Dir {
	dir := o.pathName.Dir()
	d, err := newDir(dir)
	if err != nil {
		db.DFatalf("Parent %v err %v\n", dir, err)
	}
	return d
}

func (o *Obj) SetParent(p fs.Dir) {
}

func (o *Obj) Unlink() {
}
