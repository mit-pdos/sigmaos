package memfssrv

import (
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/dir"
	"sigmaos/fs"
	"sigmaos/inode"
	"sigmaos/lockmap"
	"sigmaos/namei"
	"sigmaos/path"
	"sigmaos/port"
	"sigmaos/portclnt"
	"sigmaos/proc"
	"sigmaos/serr"
	"sigmaos/sesssrv"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
)

var rootP = path.Path{""}

//
// Servers use memfsssrv to create an in-memory file server.
// memfsssrv uses sesssrv and protsrv to handle client sigmaP
// requests.
//

type MemFs struct {
	*sesssrv.SessSrv
	ctx fs.CtxI // server context
	plt *lockmap.PathLockTable
	sc  *sigmaclnt.SigmaClnt
	pc  *portclnt.PortClnt
	pi  portclnt.PortInfo
	pn  string
}

func NewMemFsSrv(pn string, srv *sesssrv.SessSrv, sc *sigmaclnt.SigmaClnt, fencefs fs.Dir) *MemFs {
	mfs := &MemFs{
		SessSrv: srv,
		ctx:     ctx.NewCtx(sc.ProcEnv().GetUname(), 0, sp.NoClntId, nil, fencefs),
		plt:     srv.GetPathLockTable(),
		sc:      sc,
		pn:      pn,
	}
	return mfs
}

func (mfs *MemFs) SigmaClnt() *sigmaclnt.SigmaClnt {
	return mfs.sc
}

func (mfs *MemFs) MyAddrsPublic(net string) sp.Taddrs {
	return port.NewPublicAddrs(mfs.pi.Hip, mfs.pi.Pb, net, mfs.MyAddr())
}

// Note: NewDev() sets parent
func (mfs *MemFs) NewDevInode() *inode.Inode {
	return inode.NewInode(mfs.ctx, sp.DMDEVICE, nil)
}

func (mfs *MemFs) lookup(path path.Path, write bool) (fs.FsObj, *lockmap.PathLock, *serr.Err) {
	d, path := mfs.Root(path)
	lk := mfs.plt.Acquire(mfs.ctx, rootP, write)
	if len(path) == 0 {
		return d, lk, nil
	}
	_, lo, lk, _, err := namei.Walk(mfs.plt, mfs.ctx, d, lk, rootP, path, nil, write)
	if err != nil {
		mfs.plt.Release(mfs.ctx, lk, write)
		return nil, nil, err
	}
	return lo, lk, nil
}

func (mfs *MemFs) lookupParent(path path.Path, write bool) (fs.Dir, *lockmap.PathLock, *serr.Err) {
	lo, lk, err := mfs.lookup(path, write)
	if err != nil {
		return nil, nil, err
	}
	d := lo.(fs.Dir)
	return d, lk, nil
}

func (mfs *MemFs) NewDev(pn string, dev fs.Inode) *serr.Err {
	path := path.Split(pn)
	d, lk, err := mfs.lookupParent(path.Dir(), true)
	if err != nil {
		return err
	}
	defer mfs.plt.Release(mfs.ctx, lk, true)
	dev.SetParent(d)
	return dir.NewNod(mfs.ctx, d, path.Base(), dev)
}

func (mfs *MemFs) NewNod(pn string, i fs.Inode) *serr.Err {
	path := path.Split(pn)
	d, lk, err := mfs.lookupParent(path.Dir(), true)
	if err != nil {
		return err
	}
	defer mfs.plt.Release(mfs.ctx, lk, true)
	return dir.NewNod(mfs.ctx, d, path.Base(), i)
}

func (mfs *MemFs) Create(pn string, p sp.Tperm, m sp.Tmode, lid sp.TleaseId) (fs.FsObj, *serr.Err) {
	path := path.Split(pn)
	d, lk, err := mfs.lookupParent(path.Dir(), true)
	if err != nil {
		return nil, err
	}
	defer mfs.plt.Release(mfs.ctx, lk, true)
	return d.Create(mfs.ctx, path.Base(), p, m, lid, sp.NoFence())
}

func (mfs *MemFs) Remove(pn string) *serr.Err {
	path := path.Split(pn)
	d, lk, err := mfs.lookupParent(path.Dir(), true)
	if err != nil {
		return err
	}
	defer mfs.plt.Release(mfs.ctx, lk, true)
	return d.Remove(mfs.ctx, path.Base(), sp.NoFence())
}

func (mfs *MemFs) Open(pn string, m sp.Tmode, write bool) (fs.FsObj, *serr.Err) {
	path := path.Split(pn)
	lo, lk, err := mfs.lookup(path, write)
	if err != nil {
		return nil, err
	}
	mfs.plt.Release(mfs.ctx, lk, write)
	return lo, nil
}

func (mfs *MemFs) MemFsExit(status *proc.Status) error {
	if mfs.pn != "" {
		// XXX remove mount
	}
	return mfs.sc.ClntExit(status)
}

func (mfs *MemFs) Dump() error {
	d, path := mfs.Root(rootP)
	s, err := d.(*dir.DirImpl).Dump()
	if err != nil {
		return err
	}
	db.DPrintf("MEMFSSRV", "Dump: %v %v", path, s)
	return nil
}
