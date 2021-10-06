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
	"ulambda/fsclnt"
	"ulambda/fslibsrv"
	"ulambda/fssrv"
	"ulambda/inode"
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
	root  fs.Dir
	mount string
}

func MakeFsUx(mount string, pid string) *FsUx {
	ip, err := fsclnt.LocalIP()
	if err != nil {
		log.Fatalf("LocalIP %v %v\n", named.UX, err)
	}
	return MakeReplicatedFsUx(mount, ip+":0", pid, nil)
}

func MakeReplicatedFsUx(mount string, addr string, pid string, config repl.Config) *FsUx {
	// seccomp.LoadFilter()  // sanity check: if enabled we want fsux to fail
	fsux := &FsUx{}
	fsux.ch = make(chan bool)
	fsux.root = makeDir([]string{mount}, np.DMDIR, nil)
	srv, fsl, err := fslibsrv.MakeReplSrvFsLib(fsux, fsux.root, addr, named.UX, "ux", config)
	if err != nil {
		log.Fatalf("MakeSrvFsLib %v\n", err)
	}
	fsux.fssrv = srv
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
	*inode.Inode
	mu   sync.Mutex
	path []string
	ino  uint64 // Unix inode
	sz   np.Tlength
	init bool
}

func makeObj(path []string, t np.Tperm, d *Dir) *Obj {
	o := &Obj{}
	o.Inode = inode.MakeInode("", t, d)
	o.path = path
	return o
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

func (o *Obj) Inum() uint64 {
	if !o.init {
		o.stat()
	}
	return o.ino
}

func (o *Obj) Qid() np.Tqid {
	if !o.init {
		o.stat()
	}
	return np.MakeQid(np.Qtype(o.Perm()>>np.QTYPESHIFT),
		np.TQversion(0), np.Tpath(o.ino))
}

func (o *Obj) Size() np.Tlength {
	if !o.init {
		o.stat()
	}
	return o.sz
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
	st.Mode = o.Inode.Perm() | np.Tperm(0777)
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
