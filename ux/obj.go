package fsux

import (
	"fmt"
	ufs "io/fs"
	"os"
	"syscall"

	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
)

func ustat(path np.Path) (*np.Stat, *np.Err) {
	fi, err := os.Stat(path.String())
	if err != nil {
		return nil, np.MkErr(np.TErrError, err)
	}
	// to get unix ino#
	ustat, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, np.MkErr(np.TErrError, "Not a syscall.Stat_t")
	}
	st := &np.Stat{}
	st.Name = path.Base()
	st.Mode = np.Tperm(fi.Mode() & ufs.ModePerm)
	if fi.IsDir() {
		st.Mode |= np.DMDIR
	}
	// XXX version: maybe use xattr? or at least nsec since 1970
	st.Qid = np.MakeQid(np.Qtype(st.Mode>>np.QTYPESHIFT), np.TQversion(0), np.Tpath(ustat.Ino))
	st.Length = np.Tlength(fi.Size())
	s, _ := ustat.Mtim.Unix()
	st.Mtime = uint32(s)
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

func (o *Obj) Path() string {
	p := o.path.String()
	if len(o.path) == 0 {
		p = "."
	}
	return p
}

func (o *Obj) Perm() np.Tperm {
	return o.st.Mode
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

func (o *Obj) Qid() np.Tqid {
	return o.st.Qid
}

func (o *Obj) Parent() fs.Dir {
	dir := o.path.Dir()
	d, err := makeDir(dir)
	if err != nil {
		db.DFatalf("Parent %v err %v\n", dir, err)
	}
	return d
}

func (o *Obj) Stat(ctx fs.CtxI) (*np.Stat, *np.Err) {
	db.DPrintf("UXD", "%v: Stat %v\n", ctx, o)
	return o.st, nil
}
