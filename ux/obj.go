package fsux

import (
	"log"
	"os"
	"sync"
	"syscall"

	db "ulambda/debug"
	"ulambda/fs"
	np "ulambda/ninep"
)

func ustat(path np.Path) (*syscall.Stat_t, *np.Err) {
	fileinfo, err := os.Stat(path.String())
	if err != nil {
		return nil, np.MkErr(np.TErrError, err)
	}
	ustat, ok := fileinfo.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, np.MkErr(np.TErrError, "Not a syscall.Stat_t")
	}
	return ustat, nil
}

type Obj struct {
	mu      sync.Mutex
	path    np.Path
	ino     uint64 // Unix inode
	version np.TQversion
}

func makeObj(path np.Path) (*Obj, *np.Err) {
	o := &Obj{}
	if err := o.init(path); err != nil {
		return nil, err
	}
	return o, nil
}

// Collect enough info to make a qid and set sz
func (o *Obj) init(path np.Path) *np.Err {
	ustat, err := ustat(path)
	if err != nil {
		return err
	}
	o.path = path
	o.ino = ustat.Ino
	s, _ := ustat.Mtim.Unix()
	// XXX maybe use xattr? or at least nsec since 1970
	o.version = np.TQversion(s)
	return nil
}

func (o *Obj) Path() string {
	p := o.path.String()
	if len(o.path) == 0 {
		p = "."
	}
	return p
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
	return f
}

func (o *Obj) size() np.Tlength {
	ustat, err := ustat(o.path)
	if err != nil {
		return 0
	}
	return np.Tlength(ustat.Size)
}

func (o *Obj) Inum() uint64 {
	return o.ino
}

func (o *Obj) Version() np.TQversion {
	return o.version
}

func (o *Obj) VersionInc() {
}

func (o *Obj) Qid() np.Tqid {
	return np.MakeQid(np.Qtype(o.Perm()>>np.QTYPESHIFT),
		np.TQversion(o.version), np.Tpath(o.ino))
}

// convert ux perms into np perm; maybe symlink?
func (o *Obj) Perm() np.Tperm {
	fi, error := os.Stat(o.path.String())
	if error != nil {
		log.Fatalf("Perm %v err %v\n", o.path, error)
	}
	if fi.IsDir() {
		return np.DMDIR
	}
	return 0
}

func (o *Obj) Parent() fs.Dir {
	dir := o.path.Dir()
	d, err := makeDir(dir)
	if err != nil {
		log.Fatalf("Parent %v err %v\n", dir, err)
	}
	return d
}

func (o *Obj) Stat(ctx fs.CtxI) (*np.Stat, *np.Err) {
	db.DLPrintf("UXD", "%v: Stat %v\n", ctx, o)
	ustat, err := ustat(o.path)
	if err != nil {
		return nil, err
	}
	st := &np.Stat{}
	if len(o.path) > 0 {
		st.Name = o.path[len(o.path)-1]
	} else {
		st.Name = "" // root
	}
	st.Mode = o.Perm() | np.Tperm(0777)
	st.Qid = o.Qid()
	st.Uid = ""
	st.Gid = ""
	st.Length = np.Tlength(ustat.Size)
	s, _ := ustat.Mtim.Unix()
	st.Mtime = uint32(s)
	return st, nil
}
