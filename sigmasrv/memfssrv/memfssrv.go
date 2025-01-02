// Servers use package memfsssrv to create an in-memory file server.
// memfsssrv uses protsrv to handle client sigmaP requests. The server
// itself can also create, remove, etc. files in its file system,
// which directly call into protsrv.
package memfssrv

import (
	"path/filepath"

	"sigmaos/api/fs"
	"sigmaos/ctx"
	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/serr"
	"sigmaos/sigmaclnt"
	sp "sigmaos/sigmap"
	"sigmaos/sigmasrv/memfssrv/memfs/dir"
	"sigmaos/sigmasrv/memfssrv/memfs/inode"
	"sigmaos/sigmasrv/memfssrv/sigmapsrv"
	spprotosrv "sigmaos/spproto/srv"
	"sigmaos/spproto/srv/fid"
	"sigmaos/spproto/srv/lockmap"
	"sigmaos/util/syncmap"
)

var rootP = path.Tpathname{""}

type MemFs struct {
	*sigmapsrv.SigmaPSrv
	ctx   fs.CtxI // server context
	ps    *spprotosrv.ProtSrv
	roots *roots
	sc    *sigmaclnt.SigmaClnt
	pn    string
}

type roots struct {
	roots *syncmap.SyncMap[string, sp.Tfid]
	next  sp.Tfid
}

func newRoots() *roots {
	r := &roots{roots: syncmap.NewSyncMap[string, sp.Tfid]()}
	return r
}

func (rts *roots) lookupAlloc(pn string) (sp.Tfid, bool) {
	fid, ok := rts.roots.AllocNew(pn, func(k string) sp.Tfid {
		db.DPrintf(db.MEMFSSRV, "lookupAlloc: new root %q fid %d\n", pn, rts.next+1)
		rts.next += 1
		return rts.next
	})
	db.DPrintf(db.MEMFSSRV, "lookupAlloc: root %q fid %d\n", pn, fid)
	return fid, ok
}

func NewMemFsSrv(pn string, srv *sigmapsrv.SigmaPSrv, sc *sigmaclnt.SigmaClnt, fencefs fs.Dir) *MemFs {
	mfs := &MemFs{
		SigmaPSrv: srv,
		ctx:       ctx.NewCtx(sc.ProcEnv().GetPrincipal(), nil, 0, sp.NoClntId, nil, fencefs),
		sc:        sc,
		pn:        pn,
		ps:        spprotosrv.NewProtSrv(sc.ProcEnv(), srv.ProtSrvState, nil, 0, srv.GetRootCtx, spprotosrv.AttachAllowAllToAll),
		roots:     newRoots(),
	}
	return mfs
}

func (mfs *MemFs) SigmaClnt() *sigmaclnt.SigmaClnt {
	return mfs.sc
}

// Note: NewDev() sets parent
func (mfs *MemFs) NewDevInode() *inode.Inode {
	return inode.NewInode(mfs.ctx, sp.DMDEVICE, sp.NoLeaseId)
}

// Returns fid for root and remaining path from root
func (mfs *MemFs) rootFid(pn string) (sp.Tfid, path.Tpathname, *serr.Err) {
	path, err := serr.PathSplitErr(pn)
	if err != nil {
		return sp.NoFid, path, err
	}
	root, rp, rest := mfs.Root(path)
	db.DPrintf(db.MEMFSSRV, "rootFid: %q root %v rp %q rest '%v'\n", pn, root, rp, rest)
	fid, ok := mfs.roots.lookupAlloc(rp))
	if ok {
		db.DPrintf(db.MEMFSSRV, "rootFid: %q new fid %d\n", pn, fid)
		mfs.ps.NewRootFid(fid, mfs.ctx, root, rp)
	}
	return fid, rest, nil
}

// Returns FsObj for pn and the path from the root to FsObj
func (mfs *MemFs) lookupWalk(pn string) (fs.Dir, fs.FsObj, path.Tpathname, *serr.Err) {
	fid, path, err := mfs.rootFid(pn)
	if err != nil {
		return nil, nil, nil, err
	}
	db.DPrintf(db.MEMFSSRV, "lookupWalk %q %v path %v\n", pn, fid, path)
	_, parent, _, lo, err := mfs.ps.LookupWalkParent(fid, path, false, lockmap.RLOCK)
	if err != nil {
		db.DPrintf(db.MEMFSSRV, "LookupWalk %v err %v\n", path.Dir(), err)
		return nil, nil, nil, err
	}
	return parent, lo, path, err
}

func (mfs *MemFs) MkNod(pn string, i fs.FsObj) *serr.Err {
	if _, err := mfs.CreateNod(pn, sp.DMDEVICE, sp.ORDWR, sp.NoLeaseId, i); err != nil {
		return err
	}
	return nil
}

func (mfs *MemFs) CreateNod(pn string, p sp.Tperm, m sp.Tmode, lid sp.TleaseId, o fs.FsObj) (fs.FsObj, *serr.Err) {
	_, lo, path, err := mfs.lookupWalk(filepath.Dir(pn))
	db.DPrintf(db.MEMFSSRV, "Create %q perm %v dir %v base %q o %v\n", pn, p, lo, path.Base(), o)
	_, nf, err := mfs.CreateObj(mfs.ctx, lo, path, filepath.Base(pn), p, m, lid, sp.NoFence(), o)
	if err != nil {
		db.DPrintf(db.MEMFSSRV, "Create: CreateObj %q %v err %v\n", pn, nf, err)
		return nil, err
	}
	return nf.Pobj().Obj(), nil
}

func (mfs *MemFs) Create(pn string, p sp.Tperm, m sp.Tmode, lid sp.TleaseId) (fs.FsObj, *serr.Err) {
	return mfs.CreateNod(pn, p, m, lid, nil)
}

func (mfs *MemFs) Remove(pn string) *serr.Err {
	parent, lo, path, err := mfs.lookupWalk(pn)
	if err != nil {
		return err
	}
	db.DPrintf(db.MEMFSSRV, "Remove %q %v %v\n", pn, lo, path.Base())
	return mfs.RemoveObj(mfs.ctx, parent, lo, path, sp.NoFence(), fs.DEL_EXIST)
}

func (mfs *MemFs) RemoveLease(po *fid.Pobj) *serr.Err {
	db.DPrintf(db.MEMFSSRV, "RemoveLease p %v po %v", po)
	return mfs.RemoveObj(mfs.ctx, po.Parent(), po.Obj(), po.Pathname(), sp.NoFence(), fs.DEL_EXIST)
}

func (mfs *MemFs) Open(pn string, m sp.Tmode) (fs.FsObj, *serr.Err) {
	_, lo, _, err := mfs.lookupWalk(pn)
	if err != nil {
		return nil, err
	}
	no, _, err := mfs.OpenObj(mfs.ctx, lo, m)
	return no, nil
}

// For named. fsetcd notifies named when a leased file has expired;
// delete the directory entry and possibly signal the directory watch
// of the change.
func (mfs *MemFs) Notify(pn path.Tpathname) error {
	db.DPrintf(db.WATCH, "MemFs.Notify pn %v\n", pn)
	parent, lo, _, err := mfs.lookupWalk(pn.String())
	if err != nil {
		return err
	}
	return mfs.RemoveObj(mfs.ctx, parent, lo, pn, sp.NoFence(), fs.DEL_EPHEMERAL)
}

func (mfs *MemFs) Dump() error {
	d, _, path := mfs.Root(rootP)
	s, err := d.(*dir.DirImpl).Dump()
	if err != nil {
		return err
	}
	db.DPrintf("MEMFSSRV", "Dump: %v %v", path, s)

	return nil
}
