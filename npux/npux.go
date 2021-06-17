package npux

import (
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"syscall"

	db "ulambda/debug"
	"ulambda/fslib"
	np "ulambda/ninep"
	npo "ulambda/npobjsrv"
	"ulambda/npsrv"
	"ulambda/stats"
)

type NpUx struct {
	mu    sync.Mutex
	srv   *npsrv.NpServer
	ch    chan bool
	root  *Dir
	mount string
	st    *npo.SessionTable
}

func MakeNpUx(mount string, addr string) *NpUx {
	return MakeReplicatedNpUx(mount, addr, false, "", nil)
}

func MakeReplicatedNpUx(mount string, addr string, replicated bool, relayAddr string, config *npsrv.NpServerReplConfig) *NpUx {
	npux := &NpUx{}
	npux.ch = make(chan bool)
	npux.root = npux.makeDir([]string{mount}, np.DMDIR, nil)
	npux.st = npo.MakeSessionTable()
	db.Name("npuxd")
	npux.srv = npsrv.MakeReplicatedNpServer(npux, addr, false, replicated, relayAddr, config)
	fsl := fslib.MakeFsLib("npux")
	fsl.Mkdir(fslib.UX, 0777)
	err := fsl.PostServiceUnion(npux.srv.MyAddr(), fslib.UX, npux.srv.MyAddr())
	if err != nil {
		log.Fatalf("PostServiceUnion failed %v %v\n", npux.srv.MyAddr(), err)
	}
	return npux
}

func (npux *NpUx) GetSrv() *npsrv.NpServer {
	return npux.srv
}

func (npux *NpUx) Connect(conn net.Conn) npsrv.NpAPI {
	return npo.MakeNpConn(npux, conn)
}

func (npux *NpUx) RootAttach(uname string) (npo.NpObj, npo.CtxI) {
	return npux.root, nil
}

func (npux *NpUx) Serve() {
	<-npux.ch
}

func (npux *NpUx) Done() {
	npux.ch <- true
}

func (npux *NpUx) WatchTable() *npo.WatchTable {
	return nil
}

func (npux *NpUx) ConnTable() *npo.ConnTable {
	return nil
}

func (npux *NpUx) SessionTable() *npo.SessionTable {
	return npux.st
}

func (npux *NpUx) RegisterSession(sess np.Tsession) {
	npux.st.RegisterSession(sess)
}

func (npux *NpUx) Stats() *stats.Stats {
	return nil
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

func (npux *NpUx) MakeObj(path []string, t np.Tperm, d *Dir) npo.NpObj {
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

func (o *Obj) Stat(ctx npo.CtxI) (*np.Stat, error) {
	db.DLPrintf("UXD", "%v: Stat %v\n", ctx, o)
	return o.stat()
}

func (o *Obj) Wstat(ctx npo.CtxI, st *np.Stat) error {
	return nil
}

func (o *Obj) Open(ctx npo.CtxI, m np.Tmode) error {
	return nil
}

// XXX close
func (o *Obj) Close(ctx npo.CtxI, m np.Tmode) error {
	return nil
}

func (o *Obj) Remove(ctx npo.CtxI, name string) error {
	db.DLPrintf("UXD", "%v: Remove %v %v\n", ctx, o, name)
	err := os.Remove(o.Path())
	return err
}

func (o *Obj) Rename(ctx npo.CtxI, from, to string) error {
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
