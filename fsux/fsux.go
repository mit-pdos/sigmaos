package fsux

import (
	"fmt"
	"log"
	"os"
	"path"
	"sync"
	"syscall"

	db "ulambda/debug"
	"ulambda/fs"
	"ulambda/fslib"
	fos "ulambda/fsobjsrv"
	"ulambda/fssrv"
	"ulambda/named"
	np "ulambda/ninep"
	"ulambda/repl"
	usync "ulambda/sync"
	// "ulambda/seccomp"
)

type FsUx struct {
	mu    sync.Mutex
	fssrv *fssrv.FsServer
	ch    chan bool
	root  *Dir
	mount string
}

func MakeFsUx(mount string, addr string, pid string) *FsUx {
	return MakeReplicatedFsUx(mount, addr, pid, nil)
}

func MakeReplicatedFsUx(mount string, addr string, pid string, config repl.Config) *FsUx {
	// seccomp.LoadFilter()  // sanity check: if enabled we want fsux to fail
	fsux := &FsUx{}
	fsux.ch = make(chan bool)
	fsux.root = fsux.makeDir([]string{mount}, np.DMDIR, nil)
	db.Name("fsuxd")
	fsux.fssrv = fssrv.MakeFsServer(fsux, fsux.root, addr, fos.MakeProtServer(), config)
	fsl := fslib.MakeFsLib("fsux")
	fsl.Mkdir(named.UX, 0777)
	err := fsl.PostServiceUnion(fsux.fssrv.MyAddr(), named.UX, fsux.fssrv.MyAddr())
	if err != nil {
		log.Fatalf("PostServiceUnion failed %v %v\n", fsux.fssrv.MyAddr(), err)
	}

	if config == nil {
		fsuxStartCond := usync.MakeCond(fsl, path.Join(named.BOOT, pid), nil)
		fsuxStartCond.Destroy()
	}

	return fsux
}

func (fsux *FsUx) Serve() {
	<-fsux.ch
}

func (fsux *FsUx) Done() {
	fsux.ch <- true
}

type Obj struct {
	mu   sync.Mutex
	fsux *FsUx
	path []string
	t    np.Tperm
	ino  uint64
	sz   np.Tlength
	dir  *Dir
	init bool
}

func (fsux *FsUx) makeObjL(path []string, t np.Tperm, d *Dir) *Obj {
	o := &Obj{}
	o.fsux = fsux
	o.path = path
	o.t = t
	o.dir = d
	return o
}

func (fsux *FsUx) MakeObj(path []string, t np.Tperm, d *Dir) fs.FsObj {
	fsux.mu.Lock()
	defer fsux.mu.Unlock()
	return fsux.makeObjL(path, t, d)
}

func (o *Obj) String() string {
	s := fmt.Sprintf("p %v t %v", o.path, o.t)
	return s
}

func (o *Obj) Qid() np.Tqid {
	if !o.init {
		o.stat()
	}
	return np.MakeQid(np.Qtype(o.t>>np.QTYPESHIFT),
		np.TQversion(0), np.Tpath(o.ino))
}

func (o *Obj) Perm() np.Tperm {
	if !o.init {
		o.stat()
	}
	return o.t
}

func (o *Obj) Size() np.Tlength {
	if !o.init {
		o.stat()
	}
	return o.sz
}

func (o *Obj) Version() np.TQversion {
	return 0
}

func (o *Obj) Path() string {
	p := np.Join(o.path)
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

func (o *Obj) stat() (*np.Stat, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	fileinfo, err := os.Stat(o.Path())
	if err != nil {
		return nil, err
	}
	ustat, ok := fileinfo.Sys().(*syscall.Stat_t)
	if !ok {
		return nil, fmt.Errorf("Not a syscall.Stat_t")
	}
	o.ino = ustat.Ino
	o.sz = np.Tlength(ustat.Size)
	o.init = true

	st := &np.Stat{}
	if len(o.path) > 0 {
		st.Name = o.path[len(o.path)-1]
	} else {
		st.Name = "" // root
	}
	st.Mode = o.t | np.Tperm(0777)
	st.Qid = o.Qid()
	st.Uid = ""
	st.Gid = ""
	st.Length = o.sz
	s, _ := ustat.Mtim.Unix()
	st.Mtime = uint32(s)

	return st, nil
}

func (o *Obj) Stat(ctx fs.CtxI) (*np.Stat, error) {
	db.DLPrintf("UXD", "%v: Stat %v\n", ctx, o)
	return o.stat()
}

func (o *Obj) Wstat(ctx fs.CtxI, st *np.Stat) error {
	return nil
}

func (o *Obj) Open(ctx fs.CtxI, m np.Tmode) error {
	return nil
}

// XXX close
func (o *Obj) Close(ctx fs.CtxI, m np.Tmode) error {
	return nil
}

func (o *Obj) Remove(ctx fs.CtxI, name string) error {
	db.DLPrintf("UXD", "%v: Remove %v %v\n", ctx, o, name)
	err := os.Remove(o.Path())
	return err
}

func (o *Obj) Rename(ctx fs.CtxI, from, to string) error {
	oldPath := o.Path()
	p := o.path[:len(o.path)-1]
	d := append(p, to)
	db.DLPrintf("UXD", "%v: Rename o:%v from:%v to:%v d:%v\n", ctx, o, from, to, d)
	err := syscall.Rename(oldPath, np.Join(d))
	if err != nil {
		return err
	}
	o.path = d
	return nil
}
