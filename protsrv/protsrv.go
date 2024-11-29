package protsrv

import (
	"time"

	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/protsrv/lockmap"
	"sigmaos/protsrv/namei"
	"sigmaos/serr"
	"sigmaos/sessp"
	sp "sigmaos/sigmap"
	sps "sigmaos/sigmaprotsrv"
)

type GetRootCtxF func(*sp.Tprincipal, map[string]*sp.SecretProto, string, sessp.Tsession, sp.TclntId) (fs.Dir, fs.CtxI)
type AttachAuthF func(*sp.Tprincipal, string) error

// Each session has its own protsrv, but they share ProtSrvState
type ProtSrv struct {
	*ProtSrvState
	fm          *fidMap
	p           *sp.Tprincipal
	srvPE       *proc.ProcEnv
	sid         sessp.Tsession
	getRootCtx  GetRootCtxF
	attachAuthF AttachAuthF
}

func NewProtSrv(srvPE *proc.ProcEnv, pss *ProtSrvState, p *sp.Tprincipal, sid sessp.Tsession, grf GetRootCtxF, aaf AttachAuthF) *ProtSrv {
	ps := &ProtSrv{
		ProtSrvState: pss,
		fm:           newFidMap(),
		p:            p,
		srvPE:        srvPE,
		sid:          sid,
		getRootCtx:   grf,
		attachAuthF:  aaf,
	}
	db.DPrintf(db.PROTSRV, "NewProtSrv[%v] -> %v", p, ps)
	return ps
}

func NewProtServer(srvPE *proc.ProcEnv, pss *ProtSrvState, p *sp.Tprincipal, sid sessp.Tsession, grf GetRootCtxF, aaf AttachAuthF) sps.Protsrv {
	return NewProtSrv(srvPE, pss, p, sid, grf, aaf)
}

func (ps *ProtSrv) Version(args *sp.Tversion, rets *sp.Rversion) *sp.Rerror {
	rets.Msize = args.Msize
	rets.Version = "9P2000"
	return nil
}

func (ps *ProtSrv) NewRootFid(id sp.Tfid, ctx fs.CtxI, root fs.FsObj, pn path.Tpathname) {
	qid := ps.newQid(root.Perm(), root.Path())
	if err := ps.fm.Insert(id, newFidPath(newPobj(pn, root, ctx), 0, qid)); err != nil {
		db.DFatalf("NewRootFid err %v\n", err)
	}
}

func (ps *ProtSrv) Auth(args *sp.Tauth, rets *sp.Rauth) *sp.Rerror {
	return sp.NewRerrorSerr(serr.NewErr(serr.TErrNotSupported, "Auth"))
}

func (ps *ProtSrv) Attach(args *sp.Tattach, rets *sp.Rattach) (sp.TclntId, *sp.Rerror) {
	s := time.Now()
	p := path.Split(args.Aname)
	root, ctx := ps.getRootCtx(ps.p, args.GetSecrets(), args.Aname, ps.sid, args.TclntId())
	db.DPrintf(db.PROTSRV, "Attach p %v fid %v afid %v aname %v cid %v sid %v secrets %v", ctx.Principal(), args.Tfid(), args.Tafid(), args.Aname, args.TclntId(), ps.sid, args.Secrets)
	tree := root.(fs.FsObj)
	qid := ps.newQid(tree.Perm(), tree.Path())
	// If client and server do not belong to the same realm, check that the
	// client is authorized to attach
	if ctx.Principal().GetRealm() != ps.srvPE.GetRealm() {
		db.DPrintf(db.PROTSRV, "Attach auth check srv %v p %v", ps.srvPE.GetPrincipal(), ctx.Principal())
		if err := ps.attachAuthF(ctx.Principal(), p.String()); err != nil {
			return sp.NoClntId, sp.NewRerrorErr(err)
		}
	}
	if args.Aname != "" {
		dlk := ps.plt.Acquire(ctx, path.Tpathname{}, lockmap.RLOCK)
		_, lo, lk, rest, err := namei.Walk(ps.plt, ctx, root, dlk, path.Tpathname{}, p, nil, lockmap.RLOCK)
		defer ps.plt.Release(ctx, lk, lockmap.RLOCK)
		if len(rest) > 0 || err != nil {
			return sp.NoClntId, sp.NewRerrorSerr(err)
		}
		// insert before releasing
		ps.vt.Insert(lo.Path())
		tree = lo
		qid = ps.newQid(lo.Perm(), lo.Path())
	} else {
		// root is already in the version table; this updates
		// just the refcnt.
		ps.vt.Insert(root.Path())
	}
	if err := ps.fm.Insert(args.Tfid(), newFidPath(newPobj(p, tree, ctx), 0, qid)); err != nil {
		return sp.NoClntId, sp.NewRerrorSerr(err)
	}
	rets.Qid = qid.Proto()
	db.DPrintf(db.WALK_LAT, "ProtSrv.Attach %v %v\n", args.TclntId(), time.Since(s))
	return args.TclntId(), nil
}

// Close open files from this client and delete this client
func (ps *ProtSrv) Detach(args *sp.Tdetach, rets *sp.Rdetach) *sp.Rerror {
	fids := ps.fm.ClientFids(args.TclntId())
	db.DPrintf(db.PROTSRV, "Detach clnt %v fes %v\n", args.TclntId(), fids)
	for _, fid := range fids {
		ps.clunk(fid)
	}
	// Several threads maybe waiting in a clntcond of this
	// clnt. DeleteClnt will unblock them so that they can bail out.
	ps.cct.DeleteClnt(args.TclntId())
	return nil
}

func (ps *ProtSrv) newQidProtos(os []fs.FsObj) []*sp.TqidProto {
	var qids []*sp.TqidProto
	for _, o := range os {
		qid := ps.newQid(o.Perm(), o.Path())
		qids = append(qids, qid.Proto())
	}
	return qids
}

func (ps *ProtSrv) lookupObjLast(ctx fs.CtxI, f *Fid, names path.Tpathname, resolve bool, ltype lockmap.Tlock) (fs.FsObj, *serr.Err) {
	_, lo, lk, _, err := ps.lookupObj(ctx, f.Pobj(), names, ltype)
	ps.plt.Release(ctx, lk, ltype)
	if err != nil {
		return nil, err
	}
	if lo.Perm().IsSymlink() && resolve {
		return nil, serr.NewErr(serr.TErrNotDir, names[len(names)-1])
	}
	return lo, nil
}

// Requests that combine walk, open, and do operation in a single RPC,
// which also avoids clunking. They may fail because args.Wnames may
// contains a special path element; in that, case the client must walk
// args.Wnames.
func (ps *ProtSrv) Walk(args *sp.Twalk, rets *sp.Rwalk) *sp.Rerror {
	f, err := ps.fm.Lookup(args.Tfid())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}

	db.DPrintf(db.PROTSRV, "%v: Walk o %v args {%v} (%v)", f.Pobj().Ctx().ClntId(), f, args, len(args.Wnames))

	s := time.Now()
	os, lo, lk, rest, err := ps.lookupObj(f.Pobj().Ctx(), f.Pobj(), args.Wnames, lockmap.RLOCK)
	db.DPrintf(db.WALK_LAT, "ProtSrv.Walk %v %v lat %v\n", f.Pobj().Ctx().ClntId(), args.Wnames, time.Since(s))
	defer ps.plt.Release(f.Pobj().Ctx(), lk, lockmap.RLOCK)

	if lk != nil {
		ps.stats.IncPathString(lk.Path())
	}

	if err != nil && !err.IsMaybeSpecialElem() {
		return sp.NewRerrorSerr(err)
	}

	// let the client decide what to do with rest (when there is a rest)
	n := len(args.Wnames) - len(rest)
	p := append(f.Pobj().Pathname().Copy(), args.Wnames[:n]...)
	rets.Qids = ps.newQidProtos(os)
	qid := ps.newQid(lo.Perm(), lo.Path())
	db.DPrintf(db.PROTSRV, "%v: Walk NewFidPath fid %v p %v lo %v qid %v os %v", f.Pobj().Ctx().ClntId(), args.NewFid, p, lo, qid, os)
	if err := ps.fm.Insert(args.Tnewfid(), newFidPath(newPobj(p, lo, f.Pobj().Ctx()), 0, qid)); err != nil {
		return sp.NewRerrorSerr(err)
	}

	ps.vt.Insert(qid.Tpath())

	return nil
}

func (ps *ProtSrv) clunk(fid sp.Tfid) *sp.Rerror {
	f, err := ps.fm.LookupDel(fid)
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROTSRV, "%v: Clunk %v f %v path %q", f.Pobj().Ctx().ClntId(), fid, f, f.Pobj().Pathname())
	if f.IsOpen() { // has the fid been opened?
		f.Pobj().Obj().Close(f.Pobj().Ctx(), f.Mode())
		f.Close()
	}
	if _, err := ps.vt.Delete(f.Pobj().Obj().Path()); err != nil {
		db.DFatalf("%v: clunk %v vt del failed %v err %v\n", f.Pobj().Ctx().ClntId(), fid, f.Pobj(), err)
	}
	return nil
}

func (ps *ProtSrv) Clunk(args *sp.Tclunk, rets *sp.Rclunk) *sp.Rerror {
	return ps.clunk(args.Tfid())
}

func (ps *ProtSrv) Open(args *sp.Topen, rets *sp.Ropen) *sp.Rerror {
	f, err := ps.fm.Lookup(args.Tfid())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROTSRV, "%v: Open f %v %v", f.Pobj().Ctx().ClntId(), f, args)
	o := f.Pobj().Obj()
	no, qid, err := ps.OpenObj(f.Pobj().Ctx(), o, args.Tmode())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	f.SetMode(args.Tmode())
	if no != o {
		f.Pobj().SetObj(no)
	}
	rets.Qid = qid.Proto()
	return nil
}

func (ps *ProtSrv) Watch(args *sp.Twatch, rets *sp.Ropen) *sp.Rerror {
	f, err := ps.fm.Lookup(args.Tfid())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	pn := f.Pobj().Pathname()
	p := f.Pobj().Obj().Path()

	db.DPrintf(db.PROTSRV, "%v: Watch %v v %v %v", f.Pobj().Ctx().ClntId(), f.Pobj().Pathname(), f.Qid(), args)

	if !f.Pobj().Obj().Perm().IsDir() {
		return sp.NewRerrorSerr(serr.NewErr(serr.TErrNotDir, f.Pobj().Pathname().String()))
	}

	// Acquire path lock on the directory pn, so that no request can
	// change the directory while setting a watch on it.  to the
	// directory
	pl := ps.plt.Acquire(f.Pobj().Ctx(), pn, lockmap.WLOCK)
	defer ps.plt.Release(f.Pobj().Ctx(), pl, lockmap.WLOCK)

	v := ps.vt.GetVersion(p)
	if !sp.VEq(f.Qid().Tversion(), v) {
		return sp.NewRerrorSerr(serr.NewErr(serr.TErrVersion, v))
	}
	err = ps.wt.WaitWatch(pl, f.Pobj().Ctx().ClntId())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	return nil
}

func (ps *ProtSrv) Create(args *sp.Tcreate, rets *sp.Rcreate) *sp.Rerror {
	f, err := ps.fm.Lookup(args.Tfid())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROTSRV, "%v: Create %v v %v %v", f.Pobj().Ctx().ClntId(), args.Tfid(), f.Pobj().Pathname(), args.Name)

	qid, nf, err := ps.CreateObj(f.Pobj().Ctx(), f.Pobj().Obj(), f.Pobj().Pathname(), args.Name, args.Tperm(), args.Tmode(), args.TleaseId(), args.Tfence(), nil)
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	if ps.fm.Update(args.Tfid(), nf); err != nil {
		return sp.NewRerrorSerr(err)
	}
	rets.Qid = qid.Proto()
	return nil
}

func (ps *ProtSrv) ReadF(args *sp.TreadF, rets *sp.Rread) ([]byte, *sp.Rerror) {
	f, err := ps.fm.Lookup(args.Tfid())
	if err != nil {
		return nil, sp.NewRerrorSerr(err)
	}

	db.DPrintf(db.PROTSRV, "%v: ReadF f %v args {%v}\n", f.Pobj().Ctx().ClntId(), f, args)

	flk := ps.plt.Acquire(f.Pobj().Ctx(), f.Pobj().Pathname(), lockmap.RLOCK)
	defer ps.plt.Release(f.Pobj().Ctx(), flk, lockmap.RLOCK)

	data, err := f.Read(args.Toffset(), args.Tcount(), args.Tfence())
	if err != nil {
		return nil, sp.NewRerrorSerr(err)
	}
	rets.Count = uint32(len(data))
	return data, nil
}

func (ps *ProtSrv) WriteRead(args *sp.Twriteread, iov sessp.IoVec, rets *sp.Rread) (sessp.IoVec, *sp.Rerror) {
	f, err := ps.fm.Lookup(sp.Tfid(args.Fid))
	if err != nil {
		return nil, sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROTSRV, "%v: WriteRead %v args {%v} path %d\n", f.Pobj().Ctx().ClntId(), f.Pobj().Pathname(), args, f.Pobj().Obj().Path())
	retiov, err := f.WriteRead(iov)
	if err != nil {
		return nil, sp.NewRerrorSerr(err)
	}
	ps.vt.IncVersion(f.Pobj().Obj().Path())
	return retiov, nil
}

func (ps *ProtSrv) WriteF(args *sp.TwriteF, data []byte, rets *sp.Rwrite) *sp.Rerror {
	f, err := ps.fm.Lookup(args.Tfid())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}

	db.DPrintf(db.PROTSRV, "%v: WriteV %v args {%v} path %d\n", f.Pobj().Ctx().ClntId(), f.Pobj().Pathname(), args, f.Pobj().Obj().Path())

	n, err := f.Write(args.Toffset(), data, args.Tfence())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	rets.Count = uint32(n)
	ps.vt.IncVersion(f.Pobj().Obj().Path())
	return nil
}

// Remove for backwards compatability; SigmaOS uses RemoveFile (see
// below) instead of Remove, but proxy will use it.
func (ps *ProtSrv) Remove(args *sp.Tremove, rets *sp.Rremove) *sp.Rerror {
	f, err := ps.fm.Lookup(args.Tfid())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROTSRV, "%v: Remove %v", f.Pobj().Ctx().ClntId(), f.Pobj().Pathname())
	if err := ps.RemoveObj(f.Pobj().Ctx(), f.Pobj().Obj(), f.Pobj().Pathname(), args.Tfence(), fs.DEL_EXIST); err != nil {
		return sp.NewRerrorSerr(err)
	}
	return nil
}

func (ps *ProtSrv) Stat(args *sp.Trstat, rets *sp.Rrstat) *sp.Rerror {
	f, err := ps.fm.Lookup(args.Tfid())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROTSRV, "%v: Stat %v", f.Pobj().Ctx().ClntId(), f)
	ps.stats.IncPathString(f.Pobj().Pathname().String())
	o := f.Pobj().Obj()
	st, r := o.Stat(f.Pobj().Ctx())
	if r != nil {
		return sp.NewRerrorSerr(r)
	}
	rets.Stat = st.StatProto()
	return nil
}

//
// Rename: within the same directory (Wstat) and rename across directories
//

func (ps *ProtSrv) Wstat(args *sp.Twstat, rets *sp.Rwstat) *sp.Rerror {
	f, err := ps.fm.Lookup(args.Tfid())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROTSRV, "%v: Wstat %v %v", f.Pobj().Ctx().ClntId(), f, args)
	if args.Stat.Name != "" {
		// update Name atomically with rename
		if err := ps.RenameObj(f.Pobj(), args.Stat.Name, args.Tfence()); err != nil {
			return sp.NewRerrorSerr(err)
		}
	}
	// XXX ignore other Wstat for now
	return nil
}

func (ps *ProtSrv) Renameat(args *sp.Trenameat, rets *sp.Rrenameat) *sp.Rerror {
	oldf, err := ps.fm.Lookup(args.Toldfid())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	newf, err := ps.fm.Lookup(args.Tnewfid())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROTSRV, "%v: Renameat %v %v %v", oldf.Pobj().Ctx().ClntId(), oldf, newf, args)
	oo := oldf.Pobj().Obj()
	no := newf.Pobj().Obj()
	switch d1 := oo.(type) {
	case fs.Dir:
		d2, ok := no.(fs.Dir)
		if !ok {
			return sp.NewRerrorSerr(serr.NewErr(serr.TErrNotDir, newf.Pobj().Pathname()))
		}
		if oo.Path() == no.Path() {
			return sp.NewRerrorSerr(serr.NewErr(serr.TErrInval, newf.Pobj().Pathname()))
		}
		err := ps.RenameAtObj(oldf.Pobj(), newf.Pobj(), d1, d2, args.OldName, args.NewName, args.Tfence())
		if err != nil {
			return sp.NewRerrorSerr(err)
		}
	default:
		return sp.NewRerrorSerr(serr.NewErr(serr.TErrNotDir, oldf.Pobj().Pathname()))
	}
	return nil
}

func (ps *ProtSrv) LookupWalk(fid sp.Tfid, wnames path.Tpathname, resolve bool, ltype lockmap.Tlock) (*Fid, path.Tpathname, fs.FsObj, *serr.Err) {
	f, err := ps.fm.Lookup(fid)
	if err != nil {
		return nil, nil, nil, err
	}
	lo := f.Pobj().Obj()
	fname := append(f.Pobj().Pathname(), wnames...)
	if len(wnames) > 0 {
		lo, err = ps.lookupObjLast(f.Pobj().Ctx(), f, wnames, resolve, ltype)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	return f, fname, lo, nil
}

func (ps *ProtSrv) lookupWalkOpen(fid sp.Tfid, wnames path.Tpathname, resolve bool, mode sp.Tmode, ltype lockmap.Tlock) (*Fid, path.Tpathname, fs.FsObj, fs.File, *serr.Err) {
	f, fname, lo, err := ps.LookupWalk(fid, wnames, resolve, ltype)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	no, err := lo.Open(f.Pobj().Ctx(), mode)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	if no != nil {
		lo = no
	}
	i, err := fs.Obj2File(lo, fname)
	if err != nil {
		lo.Close(f.Pobj().Ctx(), mode)
		return nil, nil, nil, nil, err
	}
	return f, fname, lo, i, nil
}

func (ps *ProtSrv) RemoveFile(args *sp.Tremovefile, rets *sp.Rremove) *sp.Rerror {
	f, fname, lo, err := ps.LookupWalk(args.Tfid(), args.Wnames, args.Resolve, lockmap.WLOCK)
	if err != nil {
		db.DPrintf(db.PROTSRV, "RemoveFile %v err %v", args, err)
		return sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROTSRV, "%v: RemoveFile %v %v %v", f.Pobj().Ctx().ClntId(), f.Pobj().Pathname(), fname, args.Fid)
	if err := ps.RemoveObj(f.Pobj().Ctx(), lo, fname, args.Tfence(), fs.DEL_EXIST); err != nil {
		return sp.NewRerrorSerr(err)
	}
	return nil
}

func (ps *ProtSrv) GetFile(args *sp.Tgetfile, rets *sp.Rread) ([]byte, *sp.Rerror) {
	if args.Tcount() > sp.MAXGETSET {
		return nil, sp.NewRerrorSerr(serr.NewErr(serr.TErrInval, "too large"))
	}
	f, pn, lo, i, err := ps.lookupWalkOpen(args.Tfid(), args.Wnames, args.Resolve, args.Tmode(), lockmap.RLOCK)
	if err != nil {
		return nil, sp.NewRerrorSerr(err)
	}
	ps.stats.IncPathString(f.Pobj().Pathname().String())
	db.DPrintf(db.PROTSRV, "GetFile f %v args {%v} %v %v", f.Pobj().Ctx().ClntId(), args, pn, i)
	data, err := i.Read(f.Pobj().Ctx(), args.Toffset(), args.Tcount(), args.Tfence())
	if err != nil {
		return nil, sp.NewRerrorSerr(err)
	}
	if err := lo.Close(f.Pobj().Ctx(), args.Tmode()); err != nil {
		return nil, sp.NewRerrorSerr(err)
	}
	return data, nil
}

// Caller holds pathname lock for f
func (ps *ProtSrv) lookupPathOpen(f *Fid, dir fs.Dir, name string, mode sp.Tmode, resolve bool) (fs.FsObj, *serr.Err) {
	_, lo, _, err := dir.LookupPath(f.Pobj().Ctx(), path.Tpathname{name})
	if err != nil {
		return nil, err
	}
	if lo.Perm().IsSymlink() && resolve {
		return nil, serr.NewErr(serr.TErrNotDir, name)
	}
	no, err := lo.Open(f.Pobj().Ctx(), mode)
	if err != nil {
		return nil, err
	}
	if no != nil {
		lo = no
	}
	return lo, nil
}

// Create file or open file, and write data to it
func (ps *ProtSrv) PutFile(args *sp.Tputfile, data []byte, rets *sp.Rwrite) *sp.Rerror {
	db.DPrintf(db.PROTSRV, "%v: PutFile start args {%v}", ps.sid, args)
	if sp.Tsize(len(data)) > sp.MAXGETSET {
		return sp.NewRerrorSerr(serr.NewErr(serr.TErrInval, "too large"))
	}
	f, err := ps.fm.Lookup(args.Tfid())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROTSRV, "%v: PutFile o %v args {%v}", f.Pobj().Ctx().ClntId(), f, args)
	fn := append(f.Pobj().Pathname(), args.Wnames...)
	dname := f.Pobj().Pathname().Dir()
	lo := f.Pobj().Obj()
	var dlk, flk *lockmap.PathLock
	if len(args.Wnames) > 0 {
		// walk to directory
		f, dname, lo, err = ps.LookupWalk(args.Tfid(), args.Wnames[0:len(args.Wnames)-1], false, lockmap.WLOCK)
		if err != nil {
			return sp.NewRerrorSerr(err)
		}

		if !lo.Perm().IsDir() {
			return sp.NewRerrorSerr(serr.NewErr(serr.TErrNotDir, dname))
		}
		dlk = ps.plt.Acquire(f.Pobj().Ctx(), dname, lockmap.WLOCK)
		defer ps.plt.Release(f.Pobj().Ctx(), dlk, lockmap.WLOCK)

		db.DPrintf(db.PROTSRV, "%v: PutFile try to create %v", f.Pobj().Ctx().ClntId(), fn)
		// try to create file, which will fail if it exists
		dir := lo.(fs.Dir)
		lo, flk, err = ps.createObj(f.Pobj().Ctx(), dir, dlk, fn, args.Tperm(), args.Tmode(), args.TleaseId(), args.Tfence(), nil)
		if err != nil {
			if err.Code() != serr.TErrExists {
				return sp.NewRerrorSerr(err)
			}
			if err.Code() == serr.TErrExists && args.Tmode()&sp.OEXCL == sp.OEXCL {
				return sp.NewRerrorSerr(err)
			}
			db.DPrintf(db.PROTSRV, "%v: PutFile lookup %v", f.Pobj().Ctx().ClntId(), fn.Base())
			// look up the file and get a lock on it. note: it cannot have
			// been removed since the failed create above, because PutFile
			// holds the directory lock.
			lo, err = ps.lookupPathOpen(f, dir, fn.Base(), args.Tmode(), args.Resolve)
			if err != nil {
				return sp.NewRerrorSerr(err)
			}
			// flk also ensures that two writes execute atomically
			flk = ps.plt.Acquire(f.Pobj().Ctx(), fn, lockmap.WLOCK)
		}
	} else {
		db.DPrintf(db.PROTSRV, "%v: PutFile open %v (%v)", f.Pobj().Ctx().ClntId(), fn, dname)
		dlk = ps.plt.Acquire(f.Pobj().Ctx(), dname, lockmap.WLOCK)
		defer ps.plt.Release(f.Pobj().Ctx(), dlk, lockmap.WLOCK)
		flk = ps.plt.Acquire(f.Pobj().Ctx(), fn, lockmap.WLOCK)
		no, err := lo.Open(f.Pobj().Ctx(), args.Tmode())
		if err != nil {
			return sp.NewRerrorSerr(err)
		}
		if no != nil {
			lo = no
		}
	}
	defer ps.plt.Release(f.Pobj().Ctx(), flk, lockmap.WLOCK)

	// make an fid for the file (in case we created it)
	qid := ps.newQid(lo.Perm(), lo.Path())
	f = ps.newFid(f.Pobj().Ctx(), dname, fn.Base(), lo, args.TleaseId(), qid)
	i, err := fs.Obj2File(lo, fn)
	if err != nil {
		return sp.NewRerrorSerr(err)
	}

	ps.stats.IncPathString(f.Pobj().Pathname().String())

	if args.Tmode()&sp.OAPPEND == sp.OAPPEND && args.Toffset() != sp.NoOffset {
		return sp.NewRerrorSerr(serr.NewErr(serr.TErrInval, "offset should be sp.NoOffset"))
	}
	if args.Toffset() == sp.NoOffset && args.Tmode()&sp.OAPPEND != sp.OAPPEND {
		return sp.NewRerrorSerr(serr.NewErr(serr.TErrInval, "mode shouldbe OAPPEND"))
	}

	n, err := i.Write(f.Pobj().Ctx(), args.Toffset(), data, args.Tfence())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	err = lo.Close(f.Pobj().Ctx(), args.Tmode())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	rets.Count = uint32(n)
	return nil
}
