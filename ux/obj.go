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
	np "sigmaos/ninep"
)

func statxTimestampToTime(sts unix.StatxTimestamp) time.Time {
	return time.Unix(sts.Sec, int64(sts.Nsec))
}

func mkQid(mode np.Tperm, v np.TQversion, path np.Tpath) np.Tqid {
	return np.MakeQid(np.Qtype(mode>>np.QTYPESHIFT), v, path)
}

func umode2Perm(umode uint16) np.Tperm {
	perm := np.Tperm(umode & 0777)
	switch umode & syscall.S_IFMT {
	case syscall.S_IFREG:
		// file
	case syscall.S_IFDIR:
		perm |= np.DMDIR
	case syscall.S_IFIFO:
		perm |= np.DMNAMEDPIPE
	case syscall.S_IFLNK:
		perm |= np.DMSYMLINK
	}
	db.DPrintf("UXD0", "mode 0%o type 0%o perm %v", umode, umode&syscall.S_IFMT, perm)
	return perm
}

func ustat(path np.Path) (*np.Stat, *np.Err) {
	var statx unix.Statx_t
	db.DPrintf("UXD", "ustat %v\n", path)
	if error := unix.Statx(unix.AT_FDCWD, path.String(), unix.AT_SYMLINK_NOFOLLOW, unix.STATX_ALL, &statx); error != nil {
		db.DPrintf("UXD", "ustat %v err %v\n", path, error)
		return nil, UxTo9PError(error, path.Base())
	}
	st := &np.Stat{}
	st.Name = path.Base()
	st.Mode = umode2Perm(statx.Mode)
	// XXX use Btime in path?
	st.Qid = np.MakeQidPerm(st.Mode, 0, np.Tpath(statx.Ino))
	st.Length = np.Tlength(statx.Size)
	t := statxTimestampToTime(statx.Mtime)
	st.Mtime = uint32(t.Unix())
	return st, nil
}

type Obj struct {
	pathName np.Path
	path     np.Tpath
	perm     np.Tperm // XXX kill, but requires changing Perm() API
}

func (o *Obj) String() string {
	return fmt.Sprintf("pn %v p %v %v", o.pathName, o.path, o.perm)
}

func makeObj(path np.Path) (*Obj, *np.Err) {
	if st, err := ustat(path); err != nil {
		return &Obj{path, 0, np.DMSYMLINK}, err
	} else {
		return &Obj{path, st.Qid.Path, st.Mode}, nil
	}
}

func (o *Obj) Perm() np.Tperm {
	return o.perm
	// st, err := ustat(o.pathName)
	// if err != nil {
	// 	db.DPrintf("UXD", "Perm %v err %v\n", o.pathName, err)
	// 	return nil, err
	// }
	//db.DPrintf("UXD", "Perm %v st %v\n", o.pathName, st)
	//return st.Mode, nil
}

func (o *Obj) Path() np.Tpath {
	return o.path
}

func (o *Obj) PathName() string {
	p := o.pathName.String()
	if len(o.pathName) == 0 {
		p = "."
	}
	return p
}

func (o *Obj) Stat(ctx fs.CtxI) (*np.Stat, *np.Err) {
	db.DPrintf("UXD", "%v: Stat %v\n", ctx, o)
	st, err := ustat(o.pathName)
	if err != nil {
		return nil, err
	}
	return st, nil
}

func uxFlags(m np.Tmode) int {
	f := syscall.O_NOFOLLOW
	switch m & 3 {
	case np.OREAD:
		f |= syscall.O_RDONLY
	case np.ORDWR:
		f |= syscall.O_RDWR
	case np.OWRITE:
		f |= syscall.O_WRONLY
	case np.OEXEC:
		f |= syscall.O_RDONLY
	}
	if m&np.OTRUNC == np.OTRUNC {
		f |= syscall.O_TRUNC
	}
	if m&np.OAPPEND == np.OAPPEND {
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
	d, err := makeDir(dir)
	if err != nil {
		db.DFatalf("Parent %v err %v\n", dir, err)
	}
	return d
}

func (o *Obj) SetParent(p fs.Dir) {
}

func (o *Obj) Unlink() {
}

func (o *Obj) Size() (np.Tlength, *np.Err) {
	st, err := ustat(o.pathName)
	if err != nil {
		return 0, err
	}
	return st.Length, nil
}

func (o *Obj) Snapshot(fn fs.SnapshotF) []byte {
	return nil
}
