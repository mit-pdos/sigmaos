//go:build linux
// +build linux

package fsux

import (
	"fmt"
	ufs "io/fs"
	"os"
	"time"

	"golang.org/x/sys/unix"

	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
)

func statxTimestampToTime(sts unix.StatxTimestamp) time.Time {
	return time.Unix(sts.Sec, int64(sts.Nsec))
}

func mkQid(mode np.Tperm, v np.TQversion, path np.Tpath) np.Tqid {
	return np.MakeQid(np.Qtype(mode>>np.QTYPESHIFT), v, path)
}

func ustat(path np.Path) (*np.Stat, *np.Err) {
	fi, err := os.Stat(path.String())
	if err != nil {
		return nil, np.MkErr(np.TErrError, err)
	}
	var statx unix.Statx_t
	if err := unix.Statx(unix.AT_FDCWD, path.String(), unix.AT_SYMLINK_NOFOLLOW, unix.STATX_ALL, &statx); err != nil {
		return nil, np.MkErr(np.TErrError, err)
	}
	st := &np.Stat{}
	st.Name = path.Base()
	st.Mode = np.Tperm(fi.Mode() & ufs.ModePerm)
	if fi.IsDir() {
		st.Mode |= np.DMDIR
	}
	if fi.Mode()&ufs.ModeNamedPipe != 0 {
		st.Mode |= np.DMNAMEDPIPE
	}
	// XXX use Btime in path?
	st.Qid = np.MakeQidPerm(st.Mode, 0, np.Tpath(statx.Ino))
	st.Length = np.Tlength(fi.Size())
	t := statxTimestampToTime(statx.Mtime)
	st.Mtime = uint32(t.Unix())
	return st, nil
}

type Obj struct {
	path np.Path
	st   *np.Stat
}

func (o *Obj) String() string {
	return fmt.Sprintf("path %v st %v %v", o.path, o.st.Qid, o.st.Length)
}

func makeObj(path np.Path) (*Obj, *np.Err) {
	if st, err := ustat(path); err != nil {
		return nil, err
	} else {
		o := &Obj{}
		o.path = path
		o.st = st
		return o, nil
	}
}

func (o *Obj) Perm() np.Tperm {
	return o.st.Mode
}

func (o *Obj) Path() np.Tpath {
	return o.qid().Path
}

func (o *Obj) PathName() string {
	p := o.path.String()
	if len(o.path) == 0 {
		p = "."
	}
	return p
}

// XXX update qid?
func (o *Obj) Stat(ctx fs.CtxI) (*np.Stat, *np.Err) {
	db.DPrintf("UXD", "%v: Stat %v\n", ctx, o)
	return o.st, nil
}

func uxFlags(m np.Tmode) int {
	f := 0
	switch m & 3 {
	case np.OREAD:
		f = os.O_RDONLY
	case np.ORDWR:
		f = os.O_RDWR
	case np.OWRITE:
		f = os.O_WRONLY
	case np.OEXEC:
		f = os.O_RDONLY
	}
	if m&np.OTRUNC == np.OTRUNC {
		f |= os.O_TRUNC
	}
	if m&np.OAPPEND == np.OAPPEND {
		f |= os.O_APPEND
		f |= os.O_WRONLY
	}
	return f
}

func (o *Obj) size() np.Tlength {
	return o.st.Length
}

func (o *Obj) qid() np.Tqid {
	return o.st.Qid
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
	dir := o.path.Dir()
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
	return o.size(), nil
}

func (o *Obj) Snapshot(fn fs.SnapshotF) []byte {
	return nil
}
