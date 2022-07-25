package protsrv

import (
	db "ulambda/debug"
	"ulambda/fid"
	"ulambda/fs"
	"ulambda/lockmap"
	np "ulambda/ninep"
)

func (ps *ProtSrv) releaseLk(pl *lockmap.PathLock) {
	if pl != nil {
		ps.plt.Release(pl)
	}
}

// namei traverses target element by element or in one LookupPath
// call, depending if the underlying file system can do a lookup for
// the complete path.
func (ps *ProtSrv) namei(ctx fs.CtxI, o fs.FsObj, src, target np.Path, os []fs.FsObj) ([]fs.FsObj, fs.FsObj, *lockmap.PathLock, np.Path, *np.Err) {
	dlk := ps.plt.Acquire(src)
	dst := src.AppendPath(target)
	ps.stats.IncPath(dst)
	var plk *lockmap.PathLock
	if len(target) > 1 {
		// lock parent directory
		plk = ps.plt.Acquire(dst.Dir())
	}
	d := o.(fs.Dir)
	nos, e, rest, err := d.LookupPath(ctx, target)
	if err != nil { // an error or perhaps a ~
		db.DPrintf("PROTSRV", "%v: dir %v: file not found %v", ctx.Uname(), d, target[0])
		ps.releaseLk(plk)
		return os, d, dlk, target, err
	}
	os = append(os, nos...)
	if len(rest) == 0 { // done?
		db.DPrintf("PROTSRV", "%v: namei %v e %v os %v", ctx.Uname(), dst, e, os)
		ews := ps.plt.Acquire(dst)
		ps.plt.Release(dlk)
		ps.releaseLk(plk)
		return os, e, ews, nil, nil
	}
	ps.releaseLk(plk)
	switch e := e.(type) {
	case fs.Dir:
		ps.plt.Release(dlk) // for "."  XXX maybe not relevant
		return ps.namei(ctx, e, src.Append(target[0]), target[1:], os)
	default: // an error or perhaps a symlink
		db.DPrintf("PROTSRV", "%v: error not dir namei %T %v %v %v %v", ctx.Uname(), e, target, d, os, target[1:])
		return os, e, dlk, target, np.MkErr(np.TErrNotDir, target[0])
	}
}

// LookupObj/namei will return an lo and a locked watch for it, even
// in error cases because the caller create a new fid anyway.
func (ps *ProtSrv) lookupObj(ctx fs.CtxI, po *fid.Pobj, target np.Path) ([]fs.FsObj, fs.FsObj, *lockmap.PathLock, np.Path, *np.Err) {
	o := po.Obj()
	if len(target) == 0 {
		ps.stats.IncPath(po.Path())
		lk := ps.plt.Acquire(po.Path())
		return nil, o, lk, nil, nil
	}
	src := po.Path().Copy()
	if !o.Perm().IsDir() {
		ps.stats.IncPath(po.Path())
		lk := ps.plt.Acquire(po.Path())
		return nil, o, lk, nil, np.MkErr(np.TErrNotDir, src.Base())
	}
	return ps.namei(ctx, o, src, target, nil)
}
