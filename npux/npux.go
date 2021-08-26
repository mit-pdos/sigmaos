package npux

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
	"ulambda/kernel"
	"ulambda/netsrv"
	np "ulambda/ninep"
	usync "ulambda/sync"
	// "ulambda/seccomp"
)

type NpUx struct {
	mu    sync.Mutex
	fssrv *fssrv.FsServer
	ch    chan bool
	root  *Dir
	mount string
}

func MakeNpUx(mount string, addr string, pid string) *NpUx {
	return MakeReplicatedNpUx(mount, addr, pid, false, "", nil)
}

func MakeReplicatedNpUx(mount string, addr string, pid string, replicated bool, relayAddr string, config *netsrv.NetServerReplConfig) *NpUx {
	// seccomp.LoadFilter()  // sanity check: if enabled we want npux to fail
	npux := &NpUx{}
	npux.ch = make(chan bool)
	npux.root = npux.makeDir([]string{mount}, np.DMDIR, nil)
	db.Name("npuxd")
	npux.fssrv = fssrv.MakeFsServer(npux, npux.root, addr, fos.MakeProtServer(), replicated, relayAddr, config)
	fsl := fslib.MakeFsLib("npux")
	fsl.Mkdir(kernel.UX, 0777)
	err := fsl.PostServiceUnion(npux.fssrv.MyAddr(), kernel.UX, npux.fssrv.MyAddr())
	if err != nil {
		log.Fatalf("PostServiceUnion failed %v %v\n", npux.fssrv.MyAddr(), err)
	}

	if !replicated {
		npuxStartCond := usync.MakeCond(fsl, path.Join(kernel.BOOT, pid), nil)
		npuxStartCond.Destroy()
	}

	return npux
}

func (npux *NpUx) Serve() {
	<-npux.ch
}

func (npux *NpUx) Done() {
	npux.ch <- true
}

type Obj struct {
	mu   sync.Mutex
	npux *NpUx
	path []string
	t    np.Tperm
	ino  uint64
	sz   np.Tlength
	dir  *Dir
	init bool
}

func (npux *NpUx) makeObjL(path []string, t np.Tperm, d *Dir) *Obj {
	o := &Obj{}
	o.npux = npux
	o.path = path
	o.t = t
	o.dir = d
	return o
}

func (npux *NpUx) MakeObj(path []string, t np.Tperm, d *Dir) fs.FsObj {
	npux.mu.Lock()
	defer npux.mu.Unlock()
	return npux.makeObjL(path, t, d)
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
