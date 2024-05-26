package namei

import (
	db "sigmaos/debug"
	"sigmaos/fs"
	"sigmaos/path"
	"sigmaos/protsrv/lockmap"
	"sigmaos/serr"
)

func releaseLk(plt *lockmap.PathLockTable, ctx fs.CtxI, pl *lockmap.PathLock, ltype lockmap.Tlock) {
	if pl != nil {
		plt.Release(ctx, pl, ltype)
	}
}

// Walk traverses target element by element or in one LookupPath call,
// depending if the underlying file system can do a lookup for the
// complete path.  Caller provides locked dir.
func Walk(plt *lockmap.PathLockTable, ctx fs.CtxI, o fs.FsObj, dlk *lockmap.PathLock, dn, target path.Tpathname, os []fs.FsObj, ltype lockmap.Tlock) ([]fs.FsObj, fs.FsObj, *lockmap.PathLock, path.Tpathname, *serr.Err) {
	fn := dn.AppendPath(target)
	var plk *lockmap.PathLock
	if len(target) > 1 {
		// lock parent directory
		plk = plt.Acquire(ctx, fn.Dir(), ltype)
	}
	d := o.(fs.Dir)
	nos, e, rest, err := d.LookupPath(ctx, target)
	if err != nil { // an error or perhaps a ~
		db.DPrintf(db.NAMEI, "%v: dir %v: file not found %v", ctx.Principal(), d, target[0])
		releaseLk(plt, ctx, plk, ltype)
		return os, d, dlk, target, err
	}
	os = append(os, nos...)
	if len(rest) == 0 { // done?
		db.DPrintf(db.NAMEI, "%v: namei %v e %v os %v", ctx.Principal(), fn, e, os)
		flk := plt.Acquire(ctx, fn, ltype)
		plt.Release(ctx, dlk, ltype)
		releaseLk(plt, ctx, plk, ltype)
		return os, e, flk, nil, nil
	}
	releaseLk(plt, ctx, plk, ltype)
	switch e := e.(type) {
	case fs.Dir:
		dlk = plt.HandOverLock(ctx, dlk, target[0], ltype)
		return Walk(plt, ctx, e, dlk, dn.Append(target[0]), target[1:], os, ltype)
	default: // an error or perhaps a symlink
		db.DPrintf(db.NAMEI, "%v: error not dir namei %T %v %v %v %v", ctx.Principal(), e, target, d, os, target[1:])
		return os, e, dlk, target, serr.NewErr(serr.TErrNotDir, target[0])
	}
}
