package protsrv

import (
	db "ulambda/debug"
	"ulambda/fid"
	"ulambda/fs"
	np "ulambda/ninep"
	"ulambda/watch"
)

func (ps *ProtSrv) releasePws(pws *watch.Watch) {
	if pws != nil {
		ps.wt.Release(pws)
	}
}

// namei traverses target element by element or in one LookupPath
// call, depending if the underlying file system can do a lookup for
// the complete path.
func (ps *ProtSrv) namei(ctx fs.CtxI, o fs.FsObj, src, target np.Path, os []fs.FsObj) ([]fs.FsObj, fs.FsObj, *watch.Watch, np.Path, *np.Err) {
	dws := ps.wt.WatchLookupL(src)
	dst := src.AppendPath(target)
	ps.stats.IncPath(dst)
	var pws *watch.Watch
	if len(target) > 1 {
		// lock parent directory
		pws = ps.wt.WatchLookupL(dst.Dir())
	}
	d := o.(fs.Dir)
	nos, e, rest, err := d.LookupPath(ctx, target)
	if err != nil { // an error or perhaps a ~
		db.DPrintf("PROTSRV", "%v: dir %v: file not found %v", ctx.Uname(), d, target[0])
		ps.releasePws(pws)
		return os, d, dws, target, err
	}
	os = append(os, nos...)
	if len(rest) == 0 { // done?
		db.DPrintf("PROTSRV", "%v: namei %v e %v os %v", ctx.Uname(), dst, e, os)
		ews := ps.wt.WatchLookupL(dst)
		ps.wt.Release(dws)
		ps.releasePws(pws)
		return os, e, ews, nil, nil
	}
	ps.releasePws(pws)
	switch e := e.(type) {
	case fs.Dir:
		ps.wt.Release(dws) // for "."  XXX maybe not relevant
		return ps.namei(ctx, e, src.Append(target[0]), target[1:], os)
	default: // an error or perhaps a symlink
		db.DPrintf("PROTSRV", "%v: error not dir namei %T %v %v %v %v", ctx.Uname(), e, target, d, os, target[1:])
		return os, e, dws, target, np.MkErr(np.TErrNotDir, target[0])
	}
}

// LookupObj/namei will return an lo and a locked watch for it, even
// in error cases because the caller create a new fid anyway.
func (ps *ProtSrv) lookupObj(ctx fs.CtxI, po *fid.Pobj, target np.Path) ([]fs.FsObj, fs.FsObj, *watch.Watch, np.Path, *np.Err) {
	o := po.Obj()
	if len(target) == 0 {
		ps.stats.IncPath(po.Path())
		ws := ps.wt.WatchLookupL(po.Path())
		return nil, o, ws, nil, nil
	}
	src := po.Path().Copy()
	if !o.Perm().IsDir() {
		ps.stats.IncPath(po.Path())
		ws := ps.wt.WatchLookupL(po.Path())
		return nil, o, ws, nil, np.MkErr(np.TErrNotDir, src.Base())
	}
	return ps.namei(ctx, o, src, target, nil)
}
