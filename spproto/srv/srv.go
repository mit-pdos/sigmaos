// package spproto implements the core of sigmaos protocol. It relieas
// on file systems such as [namesrv], [proxy/ux], etc. to implement
// files, directories, etc.
package srv

import (
	"time"

	"sigmaos/api/fs"
	sps "sigmaos/api/spprotsrv"
	db "sigmaos/debug"
	"sigmaos/path"
	"sigmaos/proc"
	"sigmaos/serr"
	sessp "sigmaos/session/proto"
	sp "sigmaos/sigmap"
	"sigmaos/spproto/srv/fid"
	"sigmaos/spproto/srv/lockmap"
	"sigmaos/spproto/srv/namei"
)

type GetRootCtxF func(*sp.Tprincipal, map[string]*sp.SecretProto, string, sessp.Tsession, sp.TclntId) (fs.Dir, fs.CtxI)
type AttachAuthF func(*sp.Tprincipal, string) error

// Each session has its own protsrv, but they share ProtSrvState
type ProtSrv struct {
	*ProtSrvState
	fm          *fid.FidMap
	p           *sp.Tprincipal
	srvPE       *proc.ProcEnv
	sid         sessp.Tsession
	getRootCtx  GetRootCtxF
	attachAuthF AttachAuthF
}

func NewProtSrv(srvPE *proc.ProcEnv, pss *ProtSrvState, p *sp.Tprincipal, sid sessp.Tsession, grf GetRootCtxF, aaf AttachAuthF) *ProtSrv {
	ps := &ProtSrv{
		ProtSrvState: pss,
		fm:           fid.NewFidMap(pss.fidfl),
		p:            p,
		srvPE:        srvPE,
		sid:          sid,
		getRootCtx:   grf,
		attachAuthF:  aaf,
	}
	db.DPrintf(db.PROTSRV, "NewProtSrv[%v] -> %v", p, ps)
	return ps
}

func NewProtServer(srvPE *proc.ProcEnv, pss *ProtSrvState, p *sp.Tprincipal, sid sessp.Tsession, grf GetRootCtxF, aaf AttachAuthF) sps.ProtSrv {
	return NewProtSrv(srvPE, pss, p, sid, grf, aaf)
}

type ProtSrvStats struct {
	nfid         int
	nfidFree     int
	nlock        int
	nlockFree    int
	nversion     int
	nversionFree int
}

func (ps *ProtSrv) Stats() *ProtSrvStats {
	pss := &ProtSrvStats{}
	pss.nfid, pss.nfidFree = ps.fm.Len()
	pss.nlock, pss.nlockFree = ps.ProtSrvState.plt.Len()
	pss.nversion, pss.nversionFree = ps.ProtSrvState.vt.Len()
	return pss
}

func (ps *ProtSrv) Version(args *sp.Tversion, rets *sp.Rversion) *sp.Rerror {
	rets.Msize = args.Msize
	rets.Version = "9P2000"
	return nil
}

func (ps *ProtSrv) NewRootFid(id sp.Tfid, ctx fs.CtxI, root fs.FsObj, name string) {
	qid := ps.newQid(root.Perm(), root.Path())
	f := ps.fm.NewFid(name, root, root.(fs.Dir), ctx, 0, qid)
	if err := ps.fm.Insert(id, f); err != nil {
		db.DFatalf("NewRootFid err %v\n", err)
	}
}

func (ps *ProtSrv) Auth(args *sp.Tauth, rets *sp.Rauth) *sp.Rerror {
	return sp.NewRerrorSerr(serr.NewErr(serr.TErrNotSupported, "Auth"))
}

// A client attaches to a server using Attach. There maybe many
// clients per session. The fids of a client share the client's
// context.
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
	parent := root
	if args.Aname != "" {
		dlk := ps.plt.Acquire(ctx, tree.Path(), lockmap.RLOCK)
		os, lo, lk, rest, err := namei.Walk(ps.plt, ctx, root, dlk, p, nil, lockmap.RLOCK)
		defer ps.plt.Release(ctx, lk, lockmap.RLOCK)
		if len(rest) > 0 || err != nil {
			return sp.NoClntId, sp.NewRerrorSerr(err)
		}
		tree = lo
		parent = getParent(root, os)
		qid = ps.newQid(lo.Perm(), lo.Path())
	}
	fid := ps.fm.NewFid(p.Base(), tree, parent, ctx, 0, qid)
	if err := ps.fm.Insert(args.Tfid(), fid); err != nil {
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

func (ps *ProtSrv) lookupObjLast(ctx fs.CtxI, f *fid.Fid, names path.Tpathname, resolve bool, ltype lockmap.Tlock) (fs.FsObj, *serr.Err) {
	_, lo, lk, _, err := ps.lookupObj(ctx, f, names, ltype)
	ps.plt.Release(ctx, lk, ltype)
	if err != nil {
		return nil, err
	}
	if lo.Perm().IsSymlink() && resolve {
		return nil, serr.NewErr(serr.TErrNotDir, names[len(names)-1])
	}
	return lo, nil
}

func (ps *ProtSrv) lookupObjLastParent(ctx fs.CtxI, f *fid.Fid, names path.Tpathname, resolve bool, ltype lockmap.Tlock) (fs.Dir, fs.FsObj, *serr.Err) {
	os, lo, lk, _, err := ps.lookupObj(ctx, f, names, ltype)
	ps.plt.Release(ctx, lk, ltype)
	if err != nil {
		return nil, nil, err
	}
	parent := getParent(f.Obj().(fs.Dir), os)
	if lo.Perm().IsSymlink() && resolve {
		return nil, nil, serr.NewErr(serr.TErrNotDir, names[len(names)-1])
	}
	return parent, lo, nil
}

// Walk to args.Wnames.  Walk may fail because args.Wnames contains a
// special path element or symlink; in that, case the client must
// handle the special path element or symlink and perhaps continue the
// walk at another server.
func (ps *ProtSrv) Walk(args *sp.Twalk, rets *sp.Rwalk) *sp.Rerror {
	f, err := ps.fm.Lookup(args.Tfid())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}

	db.DPrintf(db.PROTSRV, "%v: Walk o %v args {%v} (%v)", f.Ctx().ClntId(), f, args, len(args.Wnames))

	s := time.Now()
	os, lo, lk, name, err := ps.lookupObj(f.Ctx(), f, args.Wnames, lockmap.RLOCK)

	db.DPrintf(db.WALK_LAT, "ProtSrv.Walk %v %v lat %v", f.Ctx().ClntId(), args.Wnames, time.Since(s))
	defer ps.plt.Release(f.Ctx(), lk, lockmap.RLOCK)

	if err != nil && !err.IsMaybeSpecialElem() {
		return sp.NewRerrorSerr(err)
	}

	// let the client decide what to do with rest (when there is a rest)
	rets.Qids = ps.newQidProtos(os)
	qid := ps.newQid(lo.Perm(), lo.Path())
	parent := getParent(f.Obj().(fs.Dir), os)

	db.DPrintf(db.PROTSRV, "%v: Walk NewFid fid %v lo %v qid %v os %v", f.Ctx().ClntId(), args.NewFid, lo, qid, os)

	fid := ps.fm.NewFid(name, lo, parent, f.Ctx(), 0, qid)
	if err := ps.fm.Insert(args.Tnewfid(), fid); err != nil {
		return sp.NewRerrorSerr(err)
	}
	return nil
}

func (ps *ProtSrv) clunk(fid sp.Tfid) *sp.Rerror {
	f, err := ps.fm.LookupDel(fid)
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROTSRV, "%v: Clunk %v f %v", f.Ctx().ClntId(), fid, f)
	if f.IsOpen() { // has the fid been opened?
		if _, err := ps.vt.Delete(f.Obj().Path()); err != nil {
			db.DFatalf("%v: clunk %v vt del failed %v err %v\n", f.Ctx().ClntId(), fid, f.Obj(), err)
		}
		f.Obj().Close(f.Ctx(), f.Mode())
		f.Close()
	}
	ps.fm.Free(f)
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
	db.DPrintf(db.PROTSRV, "%v: Open f %v %v", f.Ctx().ClntId(), f, args)
	o := f.Obj()
	no, qid, err := ps.OpenObj(f.Ctx(), o, args.Tmode())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	f.SetMode(args.Tmode())
	if no != o {
		f.SetObj(no)
	}

	ps.vt.Insert(qid.Tpath())

	rets.Qid = qid.Proto()
	return nil
}

func (ps *ProtSrv) Watch(args *sp.Twatch, rets *sp.Ropen) *sp.Rerror {
	f, err := ps.fm.Lookup(args.Tfid())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	p := f.Obj().Path()

	if !f.Obj().Perm().IsDir() {
		return sp.NewRerrorSerr(serr.NewErr(serr.TErrNotDir, f.Name()))
	}

	// Acquire path lock on the directory pn, so that no request can
	// change the directory while setting a watch on it.  to the
	// directory
	pl := ps.plt.Acquire(f.Ctx(), p, lockmap.WLOCK)
	defer ps.plt.Release(f.Ctx(), pl, lockmap.WLOCK)

	v := ps.vt.GetVersion(p)

	db.DPrintf(db.PROTSRV, "%v: Watch p %v pn %q qid %v %v", f.Ctx().ClntId(), f.Path(), f.Name(), f.Qid(), v)

	if !sp.VEq(f.Qid().Tversion(), v) {
		db.DPrintf(db.PROTSRV, "%v: Watch stale version p %v v %q n %v %v", f.Ctx().ClntId(), f.Path(), f.Name(), f.Qid().Tversion(), v)
		return sp.NewRerrorSerr(serr.NewErr(serr.TErrVersion, v))
	}
	err = ps.wt.WaitWatch(pl, f.Ctx().ClntId())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	return nil
}

func (ps *ProtSrv) CreateObjFm(ctx fs.CtxI, o fs.FsObj, name string, perm sp.Tperm, m sp.Tmode, lid sp.TleaseId, fence sp.Tfence, dev fs.FsObj) (sp.Tqid, *fid.Fid, *serr.Err) {
	return ps.CreateObj(ps.fm, ctx, o, name, perm, m, lid, fence, dev)
}

func (ps *ProtSrv) Create(args *sp.Tcreate, rets *sp.Rcreate) *sp.Rerror {
	f, err := ps.fm.Lookup(args.Tfid())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROTSRV, "%v: Create %v n %q args %v", f.Ctx().ClntId(), args.Tfid(), f.Name(), args.Name)

	qid, nf, err := ps.CreateObj(ps.fm, f.Ctx(), f.Obj(), args.Name, args.Tperm(), args.Tmode(), args.TleaseId(), args.Tfence(), nil)
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	if ps.fm.Update(args.Tfid(), nf); err != nil {
		return sp.NewRerrorSerr(err)
	}
	ps.fm.Free(f)
	rets.Qid = qid.Proto()
	return nil
}

func (ps *ProtSrv) ReadF(args *sp.TreadF, rets *sp.Rread) ([]byte, *sp.Rerror) {
	f, err := ps.fm.Lookup(args.Tfid())
	if err != nil {
		return nil, sp.NewRerrorSerr(err)
	}

	db.DPrintf(db.PROTSRV, "%v: ReadF f %v args {%v}\n", f.Ctx().ClntId(), f, args)

	flk := ps.plt.Acquire(f.Ctx(), f.Path(), lockmap.RLOCK)
	defer ps.plt.Release(f.Ctx(), flk, lockmap.RLOCK)

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
	db.DPrintf(db.PROTSRV, "%v: WriteRead %v args {%v} path %d\n", f.Ctx().ClntId(), f.Name(), args, f.Obj().Path())
	retiov, err := f.WriteRead(iov)
	if err != nil {
		return nil, sp.NewRerrorSerr(err)
	}
	return retiov, nil
}

func (ps *ProtSrv) WriteF(args *sp.TwriteF, data []byte, rets *sp.Rwrite) *sp.Rerror {
	f, err := ps.fm.Lookup(args.Tfid())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}

	db.DPrintf(db.PROTSRV, "%v: WriteV %v args {%v} path %d\n", f.Ctx().ClntId(), f.Name(), args, f.Obj().Path())

	n, err := f.Write(args.Toffset(), data, args.Tfence())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	rets.Count = uint32(n)
	return nil
}

// Remove for backwards compatability; SigmaOS uses RemoveFile (see
// below) instead of Remove, but proxy will use it.
func (ps *ProtSrv) Remove(args *sp.Tremove, rets *sp.Rremove) *sp.Rerror {
	f, err := ps.fm.Lookup(args.Tfid())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROTSRV, "%v: Remove %v", f.Ctx().ClntId(), f.Name())
	defer ps.clunk(args.Tfid())
	if err := ps.RemoveObj(f.Ctx(), f.Parent(), f.Obj(), f.Name(), args.Tfence(), fs.DEL_EXIST); err != nil {
		return sp.NewRerrorSerr(err)
	}
	return nil
}

func (ps *ProtSrv) Stat(args *sp.Trstat, rets *sp.Rrstat) *sp.Rerror {
	f, err := ps.fm.Lookup(args.Tfid())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROTSRV, "%v: Stat %v", f.Ctx().ClntId(), f)
	// ps.stats.IncPathString(f.Path())
	o := f.Obj()
	st, r := o.Stat(f.Ctx())
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
	db.DPrintf(db.PROTSRV, "%v: Wstat %v %v", f.Ctx().ClntId(), f, args)
	if args.Stat.Name != "" {
		// update Name atomically with rename
		if err := ps.RenameObj(f, args.Stat.Name, args.Tfence()); err != nil {
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
	db.DPrintf(db.PROTSRV, "%v: Renameat %v %v %v", oldf.Ctx().ClntId(), oldf, newf, args)
	oo := oldf.Obj()
	no := newf.Obj()
	switch d1 := oo.(type) {
	case fs.Dir:
		d2, ok := no.(fs.Dir)
		if !ok {
			return sp.NewRerrorSerr(serr.NewErr(serr.TErrNotDir, newf.Name()))
		}
		if oo.Path() == no.Path() {
			return sp.NewRerrorSerr(serr.NewErr(serr.TErrInval, newf.Name()))
		}
		_, o, lk, _, err := ps.lookupObj(oldf.Ctx(), oldf, path.Tpathname{args.OldName}, lockmap.RLOCK)
		ps.plt.Release(oldf.Ctx(), lk, lockmap.RLOCK)
		if err != nil {
			return sp.NewRerrorSerr(serr.NewErr(serr.TErrNotfound, args.OldName))
		}
		if err := ps.RenameAtObj(oldf, newf, d1, d2, args.OldName, args.NewName, o, args.Tfence()); err != nil {
			return sp.NewRerrorSerr(err)
		}
	default:
		return sp.NewRerrorSerr(serr.NewErr(serr.TErrNotDir, oldf.Name()))
	}
	return nil
}

func (ps *ProtSrv) LookupWalk(fid sp.Tfid, wnames path.Tpathname, resolve bool, ltype lockmap.Tlock) (*fid.Fid, string, fs.FsObj, *serr.Err) {
	f, err := ps.fm.Lookup(fid)
	if err != nil {
		return nil, "", nil, err
	}
	lo := f.Obj()
	fname := wnames.Base()
	if len(wnames) > 0 {
		lo, err = ps.lookupObjLast(f.Ctx(), f, wnames, resolve, ltype)
		if err != nil {
			return nil, "", nil, err
		}
	}
	return f, fname, lo, nil
}

func (ps *ProtSrv) LookupWalkParent(fid sp.Tfid, wnames path.Tpathname, resolve bool, ltype lockmap.Tlock) (*fid.Fid, fs.Dir, string, fs.FsObj, *serr.Err) {
	f, err := ps.fm.Lookup(fid)
	if err != nil {
		return nil, nil, "", nil, err
	}
	lo := f.Obj()
	parent := lo.(fs.Dir)
	fname := wnames.Base()
	if len(wnames) > 0 {
		parent, lo, err = ps.lookupObjLastParent(f.Ctx(), f, wnames, resolve, ltype)
		if err != nil {
			return nil, nil, "", nil, err
		}
	}
	return f, parent, fname, lo, nil
}

func (ps *ProtSrv) lookupWalkOpen(fid sp.Tfid, wnames path.Tpathname, resolve bool, mode sp.Tmode, ltype lockmap.Tlock) (*fid.Fid, string, fs.FsObj, fs.File, *serr.Err) {
	f, fname, lo, err := ps.LookupWalk(fid, wnames, resolve, ltype)
	if err != nil {
		return nil, "", nil, nil, err
	}
	db.DPrintf(db.PROTSRV, "%v: open %v o %v(%p)", f.Ctx().ClntId(), wnames, lo, lo)
	no, err := lo.Open(f.Ctx(), mode)
	if err != nil {
		return nil, "", nil, nil, err
	}
	if no != nil {
		lo = no
	}
	i, err := fs.Obj2File(lo, fname)
	if err != nil {
		lo.Close(f.Ctx(), mode)
		return nil, "", nil, nil, err
	}
	return f, fname, lo, i, nil
}

func (ps *ProtSrv) RemoveFile(args *sp.Tremovefile, rets *sp.Rremove) *sp.Rerror {
	f, dir, name, lo, err := ps.LookupWalkParent(args.Tfid(), args.Wnames, args.Resolve, lockmap.WLOCK)
	if err != nil {
		db.DPrintf(db.PROTSRV, "RemoveFile %v err %v", args, err)
		return sp.NewRerrorSerr(err)
	}
	db.DPrintf(db.PROTSRV, "%v: RemoveFile f %v %q %v", f.Ctx().ClntId(), f, name, args.Fid)
	if err := ps.RemoveObj(f.Ctx(), dir, lo, name, args.Tfence(), fs.DEL_EXIST); err != nil {
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
	// ps.stats.IncPathString(f.Path())
	db.DPrintf(db.PROTSRV, "GetFile f %v args {%v} %v %v", f.Ctx().ClntId(), args, pn, i)
	data, err := i.Read(f.Ctx(), args.Toffset(), args.Tcount(), args.Tfence())
	if err != nil {
		return nil, sp.NewRerrorSerr(err)
	}
	if err := lo.Close(f.Ctx(), args.Tmode()); err != nil {
		return nil, sp.NewRerrorSerr(err)
	}
	return data, nil
}

// Caller holds pathname lock for f
func (ps *ProtSrv) lookupPathOpen(f *fid.Fid, dir fs.Dir, name string, mode sp.Tmode, resolve bool) (fs.FsObj, *serr.Err) {
	_, lo, _, err := dir.LookupPath(f.Ctx(), path.Tpathname{name})
	if err != nil {
		return nil, err
	}
	if lo.Perm().IsSymlink() && resolve {
		return nil, serr.NewErr(serr.TErrNotDir, name)
	}
	no, err := lo.Open(f.Ctx(), mode)
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
	db.DPrintf(db.PROTSRV, "%v: PutFile o %v args {%v}", f.Ctx().ClntId(), f, args)
	name := args.Wnames[len(args.Wnames)-1]
	lo := f.Obj()
	var dlk, flk *lockmap.PathLock
	dir := f.Parent()
	dname := f.Name()
	if len(args.Wnames) > 0 {
		// walk to directory
		f, dname, lo, err = ps.LookupWalk(args.Tfid(), args.Wnames[0:len(args.Wnames)-1], false, lockmap.WLOCK)
		if err != nil {
			return sp.NewRerrorSerr(err)
		}

		if !lo.Perm().IsDir() {
			return sp.NewRerrorSerr(serr.NewErr(serr.TErrNotDir, dname))
		}
		dlk = ps.plt.Acquire(f.Ctx(), lo.Path(), lockmap.WLOCK)
		defer ps.plt.Release(f.Ctx(), dlk, lockmap.WLOCK)

		db.DPrintf(db.PROTSRV, "%v: PutFile try to create %q %v", f.Ctx().ClntId(), name, lo)
		// try to create file, which will fail if it exists
		dir = lo.(fs.Dir)
		lo, flk, err = ps.createObj(f.Ctx(), dir, dlk, name, args.Tperm(), args.Tmode(), args.TleaseId(), args.Tfence(), nil)
		if err != nil {
			if err.Code() != serr.TErrExists {
				return sp.NewRerrorSerr(err)
			}
			if err.Code() == serr.TErrExists && args.Tmode()&sp.OEXCL == sp.OEXCL {
				return sp.NewRerrorSerr(err)
			}
			db.DPrintf(db.PROTSRV, "%v: PutFile lookup %v", f.Ctx().ClntId(), name)
			// look up the file and get a lock on it. note: it cannot have
			// been removed since the failed create above, because PutFile
			// holds the directory lock.
			lo, err = ps.lookupPathOpen(f, dir, name, args.Tmode(), args.Resolve)
			if err != nil {
				return sp.NewRerrorSerr(err)
			}
			// flk also ensures that two writes execute atomically
			flk = ps.plt.Acquire(f.Ctx(), lo.Path(), lockmap.WLOCK)
		}
	} else {
		db.DPrintf(db.PROTSRV, "%v: PutFile open %v (%v)", f.Ctx().ClntId(), name, dname)
		dlk = ps.plt.Acquire(f.Ctx(), dir.Path(), lockmap.WLOCK)
		defer ps.plt.Release(f.Ctx(), dlk, lockmap.WLOCK)
		flk = ps.plt.Acquire(f.Ctx(), lo.Path(), lockmap.WLOCK)
		no, err := lo.Open(f.Ctx(), args.Tmode())
		if err != nil {
			return sp.NewRerrorSerr(err)
		}
		if no != nil {
			lo = no
		}
	}
	defer ps.plt.Release(f.Ctx(), flk, lockmap.WLOCK)

	// make an fid for the file (in case we created it)
	qid := ps.newQid(lo.Perm(), lo.Path())
	f = ps.newFid(ps.fm, f.Ctx(), dir, name, lo, args.TleaseId(), qid)
	i, err := fs.Obj2File(lo, name)
	if err != nil {
		return sp.NewRerrorSerr(err)
	}

	// ps.stats.IncPathString(f.Path())

	if args.Tmode()&sp.OAPPEND == sp.OAPPEND && args.Toffset() != sp.NoOffset {
		return sp.NewRerrorSerr(serr.NewErr(serr.TErrInval, "offset should be sp.NoOffset"))
	}
	if args.Toffset() == sp.NoOffset && args.Tmode()&sp.OAPPEND != sp.OAPPEND {
		return sp.NewRerrorSerr(serr.NewErr(serr.TErrInval, "mode shouldbe OAPPEND"))
	}

	n, err := i.Write(f.Ctx(), args.Toffset(), data, args.Tfence())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	err = lo.Close(f.Ctx(), args.Tmode())
	if err != nil {
		return sp.NewRerrorSerr(err)
	}
	rets.Count = uint32(n)
	return nil
}
