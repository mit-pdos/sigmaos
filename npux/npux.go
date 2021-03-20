package npux

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"syscall"

	db "ulambda/debug"
	"ulambda/fsclnt"
	"ulambda/fslib"
	np "ulambda/ninep"
	npo "ulambda/npobjsrv"
	"ulambda/npsrv"
)

type NpUx struct {
	mu    sync.Mutex
	srv   *npsrv.NpServer
	ch    chan bool
	root  npo.NpObj
	mount string
	name  string
}

func MakeNpUx(mount string) *NpUx {
	npux := &NpUx{}
	npux.ch = make(chan bool)
	npux.root = npux.MakeObj([]string{mount}, np.DMDIR, nil)
	npux.name = "npuxd:" + strconv.Itoa(os.Getpid())
	db.SetDebug()
	ip, err := fsclnt.LocalIP()
	if err != nil {
		log.Fatalf("LocalIP %v %v\n", fslib.UX, err)
	}
	npux.srv = npsrv.MakeNpServer(npux, npux.name, ip+":0")
	fsl := fslib.MakeFsLib(npux.name)
	err = fsl.PostServiceUnion(npux.srv.MyAddr(), fslib.UX, npux.srv.MyAddr())
	if err != nil {
		log.Fatalf("PostServiceUnion failed %v %v\n", npux.srv.MyAddr(), err)
	}
	return npux
}

func (npux *NpUx) Connect(conn net.Conn) npsrv.NpAPI {
	clnt := npo.MakeNpConn(npux, conn, npux.name)
	return clnt
}

func (npux *NpUx) Root() npo.NpObj {
	return npux.root
}

func (npux *NpUx) Resolver() npo.Resolver {
	return nil
}

func (npux *NpUx) Serve() {
	<-npux.ch
}

func (npux *NpUx) Done() {
	npux.ch <- true
}

type Obj struct {
	mu     sync.Mutex
	npux   *NpUx
	path   []string
	t      np.Tperm
	ino    uint64
	sz     np.Tlength
	parent npo.NpObj
	file   *os.File
	init   bool
}

func (npux *NpUx) makeObjL(path []string, t np.Tperm, p npo.NpObj) npo.NpObj {
	o := &Obj{}
	o.npux = npux
	o.path = path
	o.t = t
	o.parent = p
	return o
}

func (npux *NpUx) MakeObj(path []string, t np.Tperm, p npo.NpObj) npo.NpObj {
	npux.mu.Lock()
	defer npux.mu.Unlock()
	return npux.makeObjL(path, t, p)
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

func (o *Obj) Stat(ctx *npo.Ctx) (*np.Stat, error) {
	db.DLPrintf(o.npux.name, "UXD", "%v: Stat %v\n", ctx, o)
	return o.stat()
}

func (o *Obj) Wstat(ctx *npo.Ctx, st *np.Stat) error {
	return nil
}

func (o *Obj) uxRead(off int64, cnt int) ([]byte, error) {
	b := make([]byte, cnt)
	_, err := o.file.Seek(off, 0)
	if err != nil {
		return nil, err
	}
	n, err := o.file.Read(b)
	if err == io.EOF {
		return b[:n], nil
	}
	if err != nil {
		return nil, err
	}
	return b[:n], err
}

func (o *Obj) uxWrite(off int64, b []byte) (np.Tsize, error) {
	db.DLPrintf(o.npux.name, "UXD", "%v: WriteFile: off %v cnt %v %v\n", o, off, len(b), o.file)
	_, err := o.file.Seek(off, 0)
	if err != nil {
		return 0, err
	}
	n, err := o.file.Write(b)
	return np.Tsize(n), err
}

func (o *Obj) ReadFile(ctx *npo.Ctx, off np.Toffset, cnt np.Tsize) ([]byte, error) {
	db.DLPrintf(o.npux.name, "UXD", "%v: ReadFile: %v off %v cnt %v\n", ctx, o, off, cnt)
	b, err := o.uxRead(int64(off), int(cnt))
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (o *Obj) uxReadDir() ([]*np.Stat, error) {
	var sts []*np.Stat
	dirents, err := ioutil.ReadDir(o.Path())
	if err != nil {
		return nil, err
	}
	for _, e := range dirents {
		st := &np.Stat{}
		st.Name = e.Name()
		if e.IsDir() {
			st.Mode = np.DMDIR
		} else {
			st.Mode = 0
		}
		st.Mode = st.Mode | np.Tperm(0777)
		sts = append(sts, st)
	}
	db.DLPrintf(o.npux.name, "UXD", "%v: uxReadDir %v\n", o, sts)
	return sts, nil
}

// XXX intermediate dirs?
func (o *Obj) Lookup(ctx *npo.Ctx, p []string) ([]npo.NpObj, []string, error) {
	db.DLPrintf(o.npux.name, "UXD", "%v: Lookup %v %v\n", ctx, o, p)
	fi, err := os.Stat(np.Join(append(o.path, p...)))
	if err != nil {
		return nil, nil, err
	}
	t := np.Tperm(0)
	if fi.IsDir() {
		t = np.DMDIR
	}
	o1 := o.npux.MakeObj(append(o.path, p...), t, o)
	return []npo.NpObj{o1}, nil, nil
}

func (o *Obj) ReadDir(ctx *npo.Ctx, off np.Toffset, cnt np.Tsize) ([]*np.Stat, error) {
	db.DLPrintf(o.npux.name, "UXD", "%v: ReadDir %v %v %v\n", ctx, o, off, cnt)
	dirents, err := o.uxReadDir()
	if err != nil {
		return nil, err
	}
	return dirents, nil
}

// XXX close
func (o *Obj) Create(ctx *npo.Ctx, name string, perm np.Tperm, m np.Tmode) (npo.NpObj, error) {
	p := np.Join(append(o.path, name))
	db.DLPrintf(o.npux.name, "UXD", "%v: Create %v %v %v %v\n", ctx, o, name, p, perm)
	var err error
	var file *os.File
	if perm.IsDir() {
		err = os.Mkdir(p, os.FileMode(perm&0777))
	} else {
		file, err = os.OpenFile(p, uxFlags(m)|os.O_CREATE, os.FileMode(perm&0777))
	}
	if err != nil {
		return nil, err
	}
	o1 := o.npux.MakeObj(append(o.path, name), 0, o)
	if file != nil {
		o1.(*Obj).file = file
	}
	return o1, nil
}

// XXX close
func (o *Obj) Open(ctx *npo.Ctx, m np.Tmode) error {
	db.DLPrintf(o.npux.name, "UXD", "%v: Open %v %v\n", ctx, o, m)
	file, err := os.OpenFile(o.Path(), uxFlags(m), 0)
	if err != nil {
		return err
	}
	o.file = file
	return nil
}

func (o *Obj) Remove(ctx *npo.Ctx, name string) error {
	db.DLPrintf(o.npux.name, "UXD", "%v: Remove %v %v\n", ctx, o, name)
	err := os.Remove(o.Path())
	return err
}

func (o *Obj) Rename(ctx *npo.Ctx, from, to string) error {
	p := o.path[:len(o.path)-1]
	d := append(p, to)
	db.DLPrintf(o.npux.name, "UXD", "%v: Rename %v %v %v %v\n", ctx, o, from, to, d)
	err := syscall.Rename(o.Path(), np.Join(d))
	if err != nil {
		return err
	}
	o.path = d
	return nil
}

func (o *Obj) WriteFile(ctx *npo.Ctx, off np.Toffset, b []byte) (np.Tsize, error) {
	db.DLPrintf(o.npux.name, "UXD", "%v: WriteFile %v off %v sz %v\n", ctx, o, off, len(b))
	return o.uxWrite(int64(off), b)
}

func (o *Obj) WriteDir(ctx *npo.Ctx, off np.Toffset, b []byte) (np.Tsize, error) {
	return 0, fmt.Errorf("not supported")
}
